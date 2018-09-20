package stargazer

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

// GitHubStargazer watches a GitHub repo for a configured number of
// stargazers and calls a function when this target is reached.
type GitHubStargazer struct {
	// Interval is how often the stargazer count will be checked.
	Interval time.Duration

	// Repository is the name of the respository to watch in owner/repo format.
	Repository string

	// StargazersTarget is the number of stargazers at which the TargetHitHook
	// should be invoked.
	StargazersTarget int

	// TargetHitHook gets run when the target number of stargazers is reached,
	// or immediately if the actual number exceeds the target upon first check.
	TargetHitHook func() error

	stargazersCount int
	etag            string
	client          *http.Client
}

// Gaze starts a loop that will poll the GitHub API every interval and call
// the target hit hook if the number of stargazers reaches the configured
// target. If the stargazers count target has already been reached on the first
// check, the hook will be called.
func (sw *GitHubStargazer) Gaze() {
	log.Printf("msg=%q repo=%s target=%d",
		"watching for stargazers", sw.Repository, sw.StargazersTarget)

	t := time.NewTicker(sw.Interval)
	for ; true; <-t.C {
		lastCount := sw.StargazersCount()
		if err := sw.fetchStargazersCount(); err != nil {
			log.Printf(
				"msg=%q repo=%q err=%q",
				"error fetching stargazers count",
				sw.Repository,
				err.Error())
			continue
		}
		if sc := sw.StargazersCount(); sc >= sw.StargazersTarget && sc != lastCount {
			if err := sw.TargetHitHook(); err != nil {
				log.Printf("msg=%q repo=%q err=%q",
					"error calling stargazer target hit hook function",
					sw.Repository,
					err.Error())
			}
		}
	}
}

// StargazersCount returns the most recent number of stargazers fetched by the
// gazer.
func (sw GitHubStargazer) StargazersCount() int {
	return sw.stargazersCount
}

// fetch the most recent number of stargazers from the GitHub API and store it
// in the GitHubStargazer. 🤩 If an ETag is stored in the starwatcher, send
// it in the header to prevent repeated fetches and counting against the rate
// limit.
func (sw *GitHubStargazer) fetchStargazersCount() error {
	if sw.client == nil {
		sw.client = initHTTPClient(20 * time.Second)
	}
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s", sw.Repository)
	req, err := http.NewRequest("GET", endpoint, nil)
	req.Header.Add("Accept", "application/json")
	if sw.etag != "" {
		req.Header.Add("If-None-Match", sw.etag)
	}

	resp, err := sw.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "error reaching GitHub API")
	}
	if resp.StatusCode == http.StatusNotModified {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error during GithHub API call: %v", resp.Status)
	}
	if etag := resp.Header.Get("ETag"); etag != "" && etag != sw.etag {
		log.Printf("msg=%q etag=%s", "saving ETag", etag)
		sw.etag = etag
	}

	var apiResponse struct {
		StargazersCount int `json:"stargazers_count"`
	}
	defer resp.Body.Close()
	if err := decodeResponse(resp.Body, &apiResponse); err != nil {
		return errors.Wrap(err, "GitHub response")
	}
	log.Printf(
		"msg=%q repo=%s stargazer_count=%d",
		"setting stargazers count",
		sw.Repository,
		apiResponse.StargazersCount)

	sw.stargazersCount = apiResponse.StargazersCount
	return nil
}
