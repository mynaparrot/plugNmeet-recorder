package utils

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"google.golang.org/protobuf/proto"
)

// Notifier encapsulates the retryable HTTP client and PlugNmeet API credentials.
type Notifier struct {
	client    *retryablehttp.Client
	host      string
	apiKey    string
	apiSecret string
}

// NewNotifier creates and initializes a new Notifier instance.
// The retryablehttp.Client is created once and reused for all notifications.
func NewNotifier(host, apiKey, apiSecret string, retryMax *uint) *Notifier {
	client := retryablehttp.NewClient()
	client.Logger = nil
	if retryMax != nil {
		client.RetryMax = int(*retryMax)
	}
	return &Notifier{
		client:    client,
		host:      host,
		apiKey:    apiKey,
		apiSecret: apiSecret,
	}
}

// NotifyToPlugNmeet sends a notification to the PlugNmeet server using the configured client.
func (n *Notifier) NotifyToPlugNmeet(req *plugnmeet.RecorderToPlugNmeet) (int, error) {
	var r *retryablehttp.Request
	var err error

	body, err := proto.Marshal(req)
	if err != nil {
		return 0, err
	}
	link := fmt.Sprintf("%s/auth/recorder/notify", n.host)
	r, err = retryablehttp.NewRequest("POST", link, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	mac := hmac.New(sha256.New, []byte(n.apiSecret))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	r.Header.Set("API-KEY", n.apiKey)
	r.Header.Set("HASH-SIGNATURE", signature)
	r.Header.Set("content-type", "application/protobuf")

	resp, err := n.client.Do(r)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close() // Ensure response body is closed

	return resp.StatusCode, nil
}
