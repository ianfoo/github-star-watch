package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	stargazer "github.com/ianfoo/github-stargazer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	envTwilioAccountSID  = "TWILIO_ACCOUNT_SID"
	envTwilioAuthToken   = "TWILIO_AUTH_TOKEN"
	envTwilioPhoneNumber = "TWILIO_PHONE_NUMBER"
	envGitHubToken       = "GITHUB_TOKEN"
)

func main() {
	addr := flag.String("addr", ":4040", "Address on which to run the HTTP status server")
	log, err := logger()
	if err != nil {
		exit(err)
	}
	gazer, err := setup(log)
	if err != nil {
		exitUsage(err)
	}
	srv := setupHTTP(log, *addr, gazer)
	defer srv.Shutdown(context.Background())
	log.Infow("starting")
	gazer.Gaze()
}

func logger() (*zap.SugaredLogger, error) {
	var (
		log *zap.Logger
		err error
	)
	switch strings.ToLower(os.Getenv("ENV")) {
	case "dev", "development":
		log, err = zap.NewDevelopment()
	case "prod", "production":
		log, err = zap.NewProduction()
	default:
		log, err = zap.NewDevelopment()
	}
	if err != nil {
		return nil, err
	}
	return log.Sugar(), nil
}

func setup(log *zap.SugaredLogger) (*stargazer.GitHubStargazer, error) {
	var (
		err       error
		repo      = flag.String("repo", "", "GitHub repository to watch (owner/repo)")
		target    = flag.Uint("target", 0, "Target number of stargazers")
		star      = flag.Bool("star", true, "Star repository when threshold crossed")
		exitAfter = flag.Bool("exit", false, "Exit after threshold crossed")
		phone     = flag.String("phone", "", "Phone number to send SMS to upon reaching stargazer target")
		interval  = flag.Duration("interval", time.Minute, "How often to check stargazer count")
		sender    = flag.String("sender", "", "Twilio phone number from which to send SMS messages")

		approachingThreshold = flag.Uint(
			"approach",
			0,
			"Threshold for count at which to switch over to different interval")
		approachingInterval = flag.Duration(
			"approach-interval",
			0,
			"How often to check once past the near-target threshold")
	)

	flag.Usage = usage
	flag.Parse()
	if *target == 0 {
		return nil, errors.New("target stargazers must be greater than zero")
	}
	if *repo == "" {
		return nil, errors.New("repo is required")
	}
	if *sender == "" {
		*sender = os.Getenv(envTwilioPhoneNumber)
	}
	if *interval < time.Second {
		return nil, errors.New("minimum interval is one second")
	}
	newTwilio := func() (*stargazer.TwilioSMSSender, error) {
		var (
			sid   = os.Getenv(envTwilioAccountSID)
			token = os.Getenv(envTwilioAuthToken)
		)
		if sid == "" || token == "" || *phone == "" || *sender == "" {
			log.Infow("SMS sending disabled",
				"sid_empty", sid == "",
				"token_empty", token == "",
				"phone_empty", *phone == "",
				"sender_empty", *sender == "")
			return nil, nil
		}
		return stargazer.NewTwilioSMSSender(sid, token, *sender,
			stargazer.WithTwilioLogger(log))
	}
	twilio, err := newTwilio()
	if err != nil {
		return nil, err
	}

	var gazer *stargazer.GitHubStargazer
	sendSMS := func(message string) {
		if *phone == "" {
			return
		}
		err := twilio.Send(*phone, message)
		if err != nil {
			log.Warnw("unable to send SMS", "err", err)
		}
	}
	starRepo := func() error {
		if !*star {
			return nil
		}
		if err := gazer.Star(); err != nil {
			log.Warnw("unable to star repo", "repo", gazer.Repository, "err", err)
			return err
		}
		var count int
		if count, err = gazer.FetchStargazerCount(); err != nil {
			log.Warnw("unable to fetch updated stargazer count after starring",
				"err", err)
			sendSMS(fmt.Sprintf("Hey! GitHub repo %s has been starred by you!",
				gazer.Repository))
			return nil
		}
		sendSMS(fmt.Sprintf("Hey! GitHub repo %s has been starred by you, and now has %d stars!",
			gazer.Repository, count))
		return nil
	}
	makeHook := func(gazer *stargazer.GitHubStargazer, exit bool) func() error {
		return func() error {
			sendSMS(fmt.Sprintf("Hey! GitHub repo %s has reached %d stargazers!",
				gazer.Repository, gazer.StargazersCount()))
			err := starRepo()
			if exit {
				log.Infow("exiting")
				gazer.Stop()
			}
			return err
		}
	}
	if *approachingThreshold > 0 && *approachingInterval > 0 {
		gazer, err = stargazer.NewGitHubStargazer(
			*repo,
			int(*approachingThreshold),
			*interval,
			func() error {
				// FIXME Some really ugly closure stuff going on here,
				// helper funcs have closed over "gazer" variable, so
				// the new gazer needs to be assigned to this variable
				// or else it'll end poorly. There's a much better way
				// to do this, maybe make all helpers the outputs of
				// function generator functions.
				log.Infow("reached approaching threshold: starting new gazer",
					"new_threshold", *approachingThreshold,
					"new_interval", *approachingInterval)
				oldGazer := gazer
				oldGazer.Pause()
				defer oldGazer.Stop()
				var err error
				gazer, err = stargazer.NewGitHubStargazer(
					*repo,
					int(*target),
					*approachingInterval,
					makeHook(gazer, *exitAfter),
					stargazer.WithGitHubLogger(log),
					stargazer.WithGitHubToken(os.Getenv(envGitHubToken)))
				if err != nil {
					return err
				}
				gazer.Gaze()
				return nil
			},
			stargazer.WithGitHubLogger(log),
			stargazer.WithGitHubToken(os.Getenv(envGitHubToken)))
		if err != nil {
			return nil, err
		}
		return gazer, nil
	}
	gazer, err = stargazer.NewGitHubStargazer(
		*repo,
		int(*target),
		*interval,
		makeHook(gazer, *exitAfter),
		stargazer.WithGitHubLogger(log),
		stargazer.WithGitHubToken(os.Getenv(envGitHubToken)))
	if err != nil {
		return nil, err
	}
	return gazer, nil
}

func setupHTTP(log *zap.SugaredLogger, addr string, sg *stargazer.GitHubStargazer) http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(rw, "Send requests with GET", http.StatusMethodNotAllowed)
			return
		}
		resp := struct {
			*stargazer.GitHubStargazer
			StargazersCount int `json:"stargazers_count"`
		}{
			GitHubStargazer: sg,
			StargazersCount: sg.StargazersCount(),
		}
		e := json.NewEncoder(rw)
		err := e.Encode(resp)
		if err != nil {
			http.Error(rw,
				fmt.Sprintf("error encoding response: %v", err),
				http.StatusInternalServerError)
		}
	})
	srv := http.Server{
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    30 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	go func() {
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Errorw("error running HTTP server", "err", err)
		}
		log.Infow("HTTP server stopped")
	}()
	return srv
}

func exit(err error) {
	log.SetFlags(0)
	log.SetPrefix("")
	log.Fatal(err)
}

func exitUsage(err error) {
	log.SetFlags(0)
	log.SetPrefix(filepath.Base(os.Args[0]) + ": ")
	log.Print(err)
	flag.Usage()
	os.Exit(1)
}

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(),
		`usage: %s -repo repository -target target [optional arguments]

Required arguments:
  -repo        Owner and name of GitHub repository to watch,
               in owner/repo format. E.g., ianfoo/github-stargazer
  -target      Number of stars to watch for on the selected repository.
               Must be greater than 0.

Optional arguments:
  -phone       Phone number to send an SMS to when the star threshold
               is crossed. No SMS will be sent if no phone number is provided.
  -interval    Frequency with which the repo will be checked. Defaults to 1m.
               Must be 1s or greater.
  -star        Auto-star the repository when the threshold is crossed,
               if a GitHub token is available (see environment section below).
  -sender      Twilio phone number from which to send SMS messages.
  -addr        Address on which to run the HTTP status server. Default ":4040"

  -approach          The count of stargazers at which a different polling
                     interval should be used. Ignored if -approach-interval
		     is not specified.
  -approach-interval The interval for polling GitHub once approach threshold
                     has been reached. Ignored if -approach is not specified.

environment:
  %-20s Twilio account SID. This is required for sending SMS.
  %-20s Twilio auth token. This is required for sending SMS.
  %-20s Twilio phone number. This is required for sending SMS,
                       if not set with -sender argument.
  %-20s Github personal access token. This is required for
                       auto-starring when threshold is crossed.
                       The token must have public_repo OAuth scope.
`,
		filepath.Base(os.Args[0]),
		envTwilioAccountSID,
		envTwilioAuthToken,
		envTwilioPhoneNumber,
		envGitHubToken)
}
