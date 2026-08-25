// API documents please see https://push.spug.cc/guide for reference.
package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var (
	SmsAccessToken  = "SMS_ACCESS_TOKEN"
	SmsTemplateCode = "SMS_TEMPLATE_CODE"
)

const (
	SmsApiEndPoint = "https://push.spug.cc/sms"
)

type SmsCodeRequest struct {
	To     string `json:"to"`
	Code   string `json:"code"`
	Number string `json:"number"`
}

type SmsCodeResponse struct {
	Code      int    `json:"code"`
	Msg       string `json:"msg"`
	RequestID string `json:"request_id"`
}

func SendSMSVerifyCode(accountPhone string, verifyCode string, expireInMinutes string) error {
	var temp SmsCodeResponse

	request := SmsCodeRequest{
		To:     accountPhone,
		Code:   verifyCode,
		Number: expireInMinutes,
	}
	rawReq, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("SendSMSVerifyCode: %w", err)
	}

	response, err := http.Post(
		fmt.Sprintf("%s/%s", SmsApiEndPoint, SmsAccessToken),
		"application/json",
		bytes.NewBuffer(rawReq),
	)
	if err != nil {
		return fmt.Errorf("SendSMSVerifyCode: %w", err)
	}
	defer response.Body.Close()

	rawResp, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("SendSMSVerifyCode: %w", err)
	}
	err = json.Unmarshal(rawResp, &temp)
	if err != nil {
		return fmt.Errorf("SendSMSVerifyCode: %w", err)
	}
	if temp.Code != http.StatusOK {
		return fmt.Errorf("SendSMSVerifyCode: %s", temp.Msg)
	}

	return nil
}
