package send

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

func SendDingTalkMessage(token, secret, message string) (string, error) {
	webhookURL := fmt.Sprintf("https://oapi.dingtalk.com/robot/send?access_token=%s", token)

	if secret != "" {
		timestamp := fmt.Sprintf("%d", time.Now().UnixNano()/1e6)
		stringToSign := fmt.Sprintf("%s\n%s", timestamp, secret)

		h := hmac.New(sha256.New, []byte(secret))
		h.Write([]byte(stringToSign))
		signData := h.Sum(nil)
		sign := url.QueryEscape(base64.StdEncoding.EncodeToString(signData))

		webhookURL = fmt.Sprintf("%s&timestamp=%s&sign=%s", webhookURL, timestamp, sign)
	}

	jsonData := fmt.Sprintf(`{"msgtype":"text","text":{"content":"%s"}}`, message)

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBufferString(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
