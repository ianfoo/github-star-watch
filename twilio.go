package stargazer

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// TwilioSMSSender sends SMS messages.
type TwilioSMSSender struct {
	AccountSID string
	AuthToken  string
	Sender     string
	client     *http.Client
}

const twilioAPIBase = "https://api.twilio.com/2010-04-01"

// Send sends message to phone number 'to' in an SMS. It logs directly, but
// this is just experimental, so forgive me.
func (tss TwilioSMSSender) Send(to, message string) error {
	values := url.Values{}
	values.Set("To", to)
	values.Set("From", tss.Sender)
	values.Set("Body", message)

	endpoint := fmt.Sprintf("%s/Accounts/%s/Messages.json", twilioAPIBase, tss.AccountSID)
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(tss.AccountSID, tss.AuthToken)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")

	if tss.client == nil {
		tss.client = initHTTPClient(20 * time.Second)
	}
	resp, err := tss.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "error reaching Twilio API")
	}

	var apiResponse struct {
		MessageSID    string `json:"sid"`
		MessageStatus string `json:"status"`
		To            string `json:"to"`
		ErrCode       int    `json:"error_code"`
		ErrMessage    string `json:"error_message"`
	}

	defer resp.Body.Close()
	if err := decodeResponse(resp.Body, &apiResponse); err != nil {
		return errors.Wrap(err, "Twilio response")
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Twilio error %d: %s", apiResponse.ErrCode, apiResponse.ErrMessage)
	}
	if isNotOKMessageStatus(apiResponse.MessageStatus) {
		return fmt.Errorf("bad message status: %s", apiResponse.MessageStatus)
	}
	log.Printf(
		"msg=%q message_sid=%s message_status=%v message_to=%s",
		"sent SMS", apiResponse.MessageSID, apiResponse.MessageStatus, apiResponse.To)

	return nil
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
