package stargazer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// GitHubStargazer watches a GitHub repo for a configured number of
// stargazers and calls a function when this target is reached.
type GitHubStargazer struct {
	// Repository is the name of the respository to watch in owner/repo format.
	Repository string

	// StargazersTarget is the number of stargazers at which the TargetHitHook
	// should be invoked.
	StargazersTarget int

	// Interval is how often the stargazer count will be checked.
	Interval time.Duration

	// ThresholdCrossedHook gets run when the target number of stargazers is reached,
	// or immediately if the actual number exceeds the target upon first check.
	ThresholdCrossedHook func() error

	stargazersCount int

	apiBaseURL string
	client     *http.Client
	etag       string
	log        *zap.SugaredLogger
}

// NewGitHubStargazer returns a new gazer to watch the number of subscribers a
// GitHub repo has, and execute hook when target is crossed.
func NewGitHubStargazer(
	repo string,
	target int,
	interval time.Duration,
	hook func() error,
	options ...func(*GitHubStargazer)) (*GitHubStargazer, error) {

	if repo == "" {
		return nil, errors.New("repository must be specified")
	}
	if target < 1 {
		return nil, errors.New("target stargazers must be at least 1")
	}
	const githubAPIBaseURL = "https://api.github.com"
	sg := &GitHubStargazer{
		Repository:           repo,
		StargazersTarget:     target,
		Interval:             interval,
		ThresholdCrossedHook: hook,
		client:               &http.Client{Timeout: 20 * time.Second},
		apiBaseURL:           githubAPIBaseURL,
		log:                  zap.NewNop().Sugar(),
	}
	for _, o := range options {
		o(sg)
	}
	return sg, nil
}

// SetHook allows the caller to set the threshold-crossed function hook
// after the GitHubStargazer has been instantiated, in case the function
// needs a reference to it.
func (sg *GitHubStargazer) SetHook(hook func() error) {
	sg.ThresholdCrossedHook = hook
}

// WithGitHubLogger is an option that can be passed to NewGitHubStargazer to
// set the *zap.SugaredLogger that the GitHubStargazer will use internally.  If
// this option is not passed to NewGitHubStargazer, a no-op log will be used
// internally.
func WithGitHubLogger(logger *zap.SugaredLogger) func(*GitHubStargazer) {
	return func(sg *GitHubStargazer) {
		sg.log = logger
	}
}

// Gaze starts a loop that will poll the GitHub API every interval and call
// the target hit hook if the number of stargazers reaches the configured
// target. If the stargazers count target has already been reached on the first
// check, the hook will be called.
func (sg *GitHubStargazer) Gaze() {
	sg.log.Infow("watching for stargazers",
		"repo", sg.Repository,
		"target", sg.StargazersTarget,
		"poll_interval", sg.Interval)

	t := time.NewTicker(sg.Interval)

	var (
		count int
		err   error
	)
	// This possibly-odd-looking for loop header causes the loop to run
	// immediately instead of having to wait for the duration of the ticker
	// to elapse first.
	for ; true; <-t.C {
		if count, err = sg.fetchStargazersCount(); err != nil {
			// TODO Interpret error; determine retriability.
			// TODO Back off if too many consecutive retriable errors
			sg.log.Errorw("error fetching stargazers count",
				"repo", sg.Repository,
				"err", err.Error())
			continue
		}
		previous := sg.updateStargazersCount(count)
		if count != previous {
			sg.log.Infow("setting stargazers count",
				"repo", sg.Repository,
				"stargazers_count", count,
				"prev_stargazers_count", previous)
		}
		if sg.didNotPassThreshold(previous, count) {
			continue
		}
		if err := sg.ThresholdCrossedHook(); err != nil {
			sg.log.Infow("error calling stargazer target hit hook function",
				"repo", sg.Repository,
				"err", err)
		}
	}
}

func (sg *GitHubStargazer) updateStargazersCount(latest int) int {
	old := sg.stargazersCount
	sg.stargazersCount = latest
	return old
}

func (sg GitHubStargazer) didNotPassThreshold(old, current int) bool {
	return current < sg.StargazersTarget || old >= current
}

// StargazersCount returns the most recent number of stargazers fetched by the
// gazer.
func (sg GitHubStargazer) StargazersCount() int {
	return sg.stargazersCount
}

// fetch the most recent number of stargazers from the GitHub API and store it
// in the GitHubStargazer. 🤩 If an ETag is stored in the starwatcher, send
// it in the header to prevent repeated fetches and counting against the rate
// limit.
func (sg *GitHubStargazer) fetchStargazersCount() (int, error) {
	if sg.client == nil {
		sg.client = &http.Client{Timeout: 20 * time.Second}
	}
	endpoint := fmt.Sprintf("%s/repos/%s", sg.apiBaseURL, sg.Repository)
	req, err := http.NewRequest("GET", endpoint, nil)
	req.Header.Add("Accept", "application/json")
	if sg.etag != "" {
		req.Header.Add("If-None-Match", sg.etag)
	}

	resp, err := sg.client.Do(req)
	if err != nil {
		return -1, errors.Wrapf(err, "error reaching GitHub API: %s", endpoint)
	}
	if resp.StatusCode == http.StatusNotModified {
		return sg.StargazersCount(), nil
	}
	if resp.StatusCode != http.StatusOK {
		return -1, fmt.Errorf("error during GithHub API call: %v (url: %s)",
			resp.Status, endpoint)
	}
	if etag := resp.Header.Get("ETag"); etag != "" && etag != sg.etag {
		sg.etag = etag
	}
	defer resp.Body.Close()
	return stargazersFromJSON(resp.Body)
}

func stargazersFromJSON(r io.Reader) (int, error) {
	var apiResponse struct {
		StargazersCount int `json:"stargazers_count"`
	}
	d := json.NewDecoder(r)
	if err := d.Decode(&apiResponse); err != nil {
		return -1, errors.Wrap(err, "error decoding GitHub JSON response")
	}
	return apiResponse.StargazersCount, nil
}
