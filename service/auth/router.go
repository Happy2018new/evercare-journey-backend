package auth

import "github.com/Happy2018new/evercare-journey-backed/service/general"

const (
	RequestTypeEmptyContext uint8 = iota
	RequestTypeFinishVerify
)

const (
	NextActionNeedsRegistry uint8 = iota
	NextActionReceiveSMS
	NextActionContinueLogin
)

type UserLoginRequest struct {
	RequestType     uint8                    `json:"request_type"`
	PhoneNumber     string                   `json:"phone_number"`
	Password        string                   `json:"password,omitempty"`
	CaptchaResponse *general.CaptchaResponse `json:"captcha_response,omitempty"`
}

type UserLoginResponse struct {
	general.BasicResponseInfo
	NextAction     uint8                   `json:"next_action"`
	RequestVerify  bool                    `json:"request_verify"`
	CaptchaRequest *general.CaptchaRequest `json:"captcha_request,omitempty"`
}
