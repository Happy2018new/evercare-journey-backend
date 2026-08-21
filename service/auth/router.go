package auth

import (
	"fmt"
	"net/http"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/gin-gonic/gin"
)

const (
	RequestTypeLoginRequest uint8 = iota
	RequestTypeFinishCaptcha
	RequestTypeSubmitSMSCode
)

const (
	NextActionFinishCaptcha uint8 = iota
	NextActionReceiveSMSCode
	NextActionNeedReLogin
	NextActionNeedReCaptcha
	NextActionNeedReSubmit
	NextActionConfigProfile
	NextActionFinishLogin
)

type UserLoginRequest struct {
	RequestType     uint8                    `json:"request_type"`
	AccountPhone    string                   `json:"phone_number,omitempty"`
	SMSVerifyCode   string                   `json:"sms_verify_code,omitempty"`
	CaptchaResponse *general.CaptchaResponse `json:"captcha_response,omitempty"`
}

type UserLoginResponse struct {
	general.BasicResponseInfo
	NextAction     uint8                   `json:"next_action"`
	UserIdentity   string                  `json:"user_identity"`
	LoginToken     string                  `json:"login_token"`
	CaptchaRequest *general.CaptchaRequest `json:"captcha_request,omitempty"`
}

func HandleLogin(c *gin.Context) {
	var request UserLoginRequest

	err := c.Bind(&request)
	if err != nil {
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(err, "HandleLogin", "无效请求"),
			),
		})
		return
	}

	switch request.RequestType {
	case RequestTypeLoginRequest:
		handleLoginRequest(c, request)
		return
	case RequestTypeFinishCaptcha:
		handleFinishCaptcha(c, request)
		return
	case RequestTypeSubmitSMSCode:
		handleSubmitSMSCode(c, request)
		return
	}

	c.JSON(http.StatusOK, UserLoginResponse{
		BasicResponseInfo: general.FromGeneralError(
			define.NewGeneralError(
				fmt.Errorf("Unsupported request type %d", request.RequestType),
				"HandleLogin",
				"无效请求",
			),
		),
	})
}

type SessionCheckRequest struct {
	general.BasicSessionInfo
}

type SessionCheckResponse struct {
	general.BasicResponseInfo
	Status uint8 `json:"status"`
}

func HandleSessionCheck(c *gin.Context) {
	var request SessionCheckRequest

	err := c.Bind(&request)
	if err != nil {
		c.JSON(http.StatusOK, SessionCheckResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(err, "HandleSessionCheck", "无效请求"),
			),
		})
		return
	}

	status, generalErr := ValidateSession(request.BasicSessionInfo)
	if generalErr != nil {
		c.JSON(http.StatusOK, SessionCheckResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("HandleSessionCheck")),
		})
		return
	}
	if status == ValidateSessionStatusValidSession {
		if generalErr = ExtendSession(request.BasicSessionInfo.UserIdentity); generalErr != nil {
			c.JSON(http.StatusOK, SessionCheckResponse{
				BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("HandleSessionCheck")),
			})
			return
		}
	}

	c.JSON(http.StatusOK, SessionCheckResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		Status:            status,
	})
}
