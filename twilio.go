package stargazer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// TwilioSMSSender sends SMS messages.
type TwilioSMSSender struct {
	//AccountSID Is the Twilio account SID.
	AccountSID string

	// AuthToken is the Twilio auth token.
	AuthToken string

	// Sender is the phone number from which the SMS messages should be sent.
	// This must be a phone number set up in your Twilio account.
	Sender string

	apiBaseURL string
	client     *http.Client
	log        *zap.SugaredLogger
}

// NewTwilioSMSSender returns a new SMS sender with the
func NewTwilioSMSSender(
	sid, authToken, sender string,
	options ...func(*TwilioSMSSender)) (*TwilioSMSSender, error) {

	if sid == "" {
		return nil, errors.New("Twilio account SID must be specified")
	}
	if authToken == "" {
		return nil, errors.New("Twilio auth token must be specified")
	}
	if sender == "" {
		return nil, errors.New("sender phone number must be specified")
	}
	const twilioAPIBaseURL = "https://api.twilio.com/2010-04-01"
	ts := &TwilioSMSSender{
		AccountSID: sid,
		AuthToken:  authToken,
		Sender:     sender,
		log:        zap.NewNop().Sugar(),
		client:     &http.Client{Timeout: 20 * time.Second},
		apiBaseURL: twilioAPIBaseURL,
	}
	for _, o := range options {
		o(ts)
	}
	return ts, nil
}

func WithTwilioLogger(logger *zap.SugaredLogger) func(*TwilioSMSSender) {
	return func(sg *TwilioSMSSender) {
		sg.log = logger
	}
}

// Send sends message to phone number 'to' in an SMS.
func (ts TwilioSMSSender) Send(to, message string) error {
	req, err := ts.makeFormRequest(to, message)
	if err != nil {
		return err
	}
	resp, err := ts.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "error reaching Twilio API")
	}

	defer resp.Body.Close()
	apiResponse, err := decodeTwilioAPIResponse(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Twilio error %d: %s", apiResponse.ErrCode, apiResponse.ErrMessage)
	}
	if isNotOKMessageStatus(apiResponse.MessageStatus) {
		return fmt.Errorf("bad message status: %s", apiResponse.MessageStatus)
	}
	ts.log.Infow("sent SMS",
		"message_sid", apiResponse.MessageSID,
		"message_status", apiResponse.MessageStatus,
		"message_to", apiResponse.To)

	return nil
}

type twilioAPIResponse struct {
	MessageSID    string `json:"sid"`
	MessageStatus string `json:"status"`
	To            string `json:"to"`
	ErrCode       int    `json:"error_code"`
	ErrMessage    string `json:"error_message"`
}

func decodeTwilioAPIResponse(r io.Reader) (*twilioAPIResponse, error) {
	response := &twilioAPIResponse{}
	d := json.NewDecoder(r)
	if err := d.Decode(response); err != nil {
		return nil, errors.Wrap(err, "error decoding Twilio response")
	}
	return response, nil
}

func (ts TwilioSMSSender) makeFormRequest(to, message string) (*http.Request, error) {
	values := url.Values{}
	values.Set("From", ts.Sender)
	values.Set("To", to)
	values.Set("Body", message)

	endpoint := fmt.Sprintf("%s/Accounts/%s/Messages.json", ts.apiBaseURL, ts.AccountSID)
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(ts.AccountSID, ts.AuthToken)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")
	return req, nil
}

func isNotOKMessageStatus(status string) bool {
	okStatuses := []string{"accepted", "queued", "sending", "delivered"}
	for _, s := range okStatuses {
		if status == s {
			return false
		}
	}
	return true
}
