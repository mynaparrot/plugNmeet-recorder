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

var (
	httpClient *retryablehttp.Client
)

func init() {
	httpClient = retryablehttp.NewClient()
	// disable default logger
	httpClient.Logger = nil
}

// NotifyToPlugNmeet will use a shared retryablehttp client to make requests.
func NotifyToPlugNmeet(host, apiKey, apiSecret string, req *plugnmeet.RecorderToPlugNmeet, retryMax *uint) (int, error) {
	// a temporary client to handle retry logic per call without affecting the shared client's settings
	tempClient := *httpClient
	if retryMax != nil {
		tempClient.RetryMax = int(*retryMax)
	} else {
		// reset to default if not provided
		tempClient.RetryMax = httpClient.RetryMax
	}

	body, err := proto.Marshal(req)
	if err != nil {
		return 0, err
	}

	link := fmt.Sprintf("%s/auth/recorder/notify", host)
	r, err := retryablehttp.NewRequest("POST", link, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	r.Header.Set("API-KEY", apiKey)
	r.Header.Set("HASH-SIGNATURE", signature)
	r.Header.Set("content-type", "application/protobuf")

	resp, err := tempClient.Do(r)
	if err != nil {
		return 0, err
	}

	return resp.StatusCode, nil
}
