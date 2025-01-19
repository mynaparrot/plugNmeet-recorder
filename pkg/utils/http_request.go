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

// NotifyToPlugNmeet will use retryablehttp to make request
func NotifyToPlugNmeet(host, apiKey, apiSecret string, req *plugnmeet.RecorderToPlugNmeet, retryMax *uint) (int, error) {
	client := retryablehttp.NewClient()
	client.Logger = nil
	if retryMax != nil {
		client.RetryMax = int(*retryMax)
	}
	var r *retryablehttp.Request
	var err error

	body, err := proto.Marshal(req)
	if err != nil {
		return 0, err
	}
	link := fmt.Sprintf("%s/auth/recorder/notify", host)
	r, err = retryablehttp.NewRequest("POST", link, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	r.Header.Set("API-KEY", apiKey)
	r.Header.Set("HASH-SIGNATURE", signature)
	r.Header.Set("content-type", "application/protobuf")

	resp, err := client.Do(r)
	if err != nil {
		return 0, err
	}

	return resp.StatusCode, nil
}
