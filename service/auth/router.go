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
				define.NewGeneralError("HandleLogin", err, define.LangKeyGeneralInvalidRequest),
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
				"HandleLogin",
				fmt.Errorf("Unsupported request type %d", request.RequestType),
				define.LangKeyGeneralInvalidRequest,
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
				define.NewGeneralError("HandleSessionCheck", err, define.LangKeyGeneralInvalidRequest),
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
