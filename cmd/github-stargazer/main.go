package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	stargazer "github.com/ianfoo/github-stargazer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	envTwilioAccountSID  = "TWILIO_ACCOUNT_SID"
	envTwilioAuthToken   = "TWILIO_AUTH_TOKEN"
	envTwilioPhoneNumber = "TWILIO_PHONE_NUMBER"
)

func main() {
	gazer, err := setup()
	if err != nil {
		exit(err)
	}
	gazer.Gaze()
}

func setup() (*stargazer.GitHubStargazer, error) {
	var (
		repo     = flag.String("repo", "", "GitHub repository to watch (owner/repo)")
		target   = flag.Uint("target", 0, "Target number of stargazers")
		phone    = flag.String("phone", "", "Phone number to send SMS to upon reaching stargazer target")
		interval = flag.Duration("interval", time.Minute, "How often to check stargazer count")
		sender   = flag.String("sender", "", "Twilio phone number from which to send SMS messages")
	)
	var log *zap.SugaredLogger
	{
		plainLog, err := zap.NewDevelopment()
		if err != nil {
			return nil, err
		}
		log = plainLog.Sugar()
	}

	flag.Parse()
	if *target == 0 {
		return nil, errors.New("target stargazers must be greater than zero")
	}
	if *phone == "" {
		return nil, errors.New("phone number is required")
	}
	if *repo == "" {
		return nil, errors.New("repo is required")
	}
	if *sender == "" {
		*sender = os.Getenv(envTwilioPhoneNumber)
	}
	twilio, err := stargazer.NewTwilioSMSSender(os.Getenv(envTwilioAccountSID),
		os.Getenv(envTwilioAuthToken),
		*sender, stargazer.WithTwilioLogger(log))
	if err != nil {
		return nil, err
	}

	gazer, err := stargazer.NewGitHubStargazer(
		*repo,
		int(*target),
		*interval,
		nil,
		stargazer.WithGitHubLogger(log))
	if err != nil {
		return nil, err
	}
	hook := func() error {
		return twilio.Send(*phone, fmt.Sprintf(
			"Hey! GitHub repo %s has reached %d stargazers!",
			gazer.Repository, gazer.StargazersCount()))
	}
	gazer.SetHook(hook)
	return gazer, nil
}

func exit(err error) {
	log.SetFlags(0)
	log.SetPrefix("")
	log.Fatal(err)
}
