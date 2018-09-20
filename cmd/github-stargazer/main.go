package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	stargazer "github.com/ianfoo/github-stargazer"
	"github.com/pkg/errors"
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
	)
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

	twilio := stargazer.TwilioSMSSender{
		AccountSID: os.Getenv(envTwilioAccountSID),
		AuthToken:  os.Getenv(envTwilioAuthToken),
		Sender:     os.Getenv(envTwilioPhoneNumber),
	}
	if twilio.AccountSID == "" || twilio.AuthToken == "" {
		return nil, fmt.Errorf(
			"set %s and %s in your environment",
			envTwilioAccountSID, envTwilioAuthToken)
	}
	if twilio.Sender == "" {
		return nil, fmt.Errorf(
			"set %s in your environment to the number that Twilio should send from",
			envTwilioPhoneNumber)
	}

	gazer := &stargazer.GitHubStargazer{
		Repository:       *repo,
		StargazersTarget: int(*target),
		Interval:         *interval,
	}
	gazer.TargetHitHook = func() error {
		return twilio.Send(*phone, fmt.Sprintf(
			"Hey! GitHub repo %s has reached %d stargazers!",
			gazer.Repository, gazer.StargazersCount()))
	}
	return gazer, nil
}

func exit(err error) {
	log.SetFlags(0)
	log.SetPrefix("")
	log.Fatal(err)
}
