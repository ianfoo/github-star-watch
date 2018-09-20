package stargazer

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

func initHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
	}
}

// decode unmarshals a JSON payload into dest, which must be a pointer.
func decodeResponse(payload io.Reader, dest interface{}) error {
	d := json.NewDecoder(payload)
	if err := d.Decode(&dest); err != nil {
		return errors.Wrap(err, "error decoding JSON body")
	}
	return nil
}
