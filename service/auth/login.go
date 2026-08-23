package auth

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/database/handle"
	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/Happy2018new/evercare-journey-backend/utils"
	"github.com/gin-gonic/gin"
)

const (
	DefaultAccountPhoneLength = 11
	DefaultSmsExpireInMinutes = "五"
)

func handleLoginRequest(c *gin.Context, request UserLoginRequest) {
	if len(request.AccountPhone) != DefaultAccountPhoneLength {
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(
					"handleLoginRequest",
					fmt.Errorf("Provided phone number must have %d characters", DefaultAccountPhoneLength),
					define.LangKeyLoginPhoneLengthErr,
					fmt.Sprintf("%d", DefaultAccountPhoneLength),
				),
			),
		})
		return
	}
	if _, err := strconv.ParseInt(DefaultSmsExpireInMinutes, 10, 64); err != nil {
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(
					"handleLoginRequest",
					fmt.Errorf("Invalid phone number %s", request.AccountPhone),
					define.LangKeyLoginPhoneInvalidErr,
				),
			),
		})
		return
	}

	captcha, generalErr := general.GenerateNewCaptchaRequest(request)
	if generalErr != nil {
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("handleLoginRequest")),
		})
		return
	}

	c.JSON(http.StatusOK, UserLoginResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		NextAction:        NextActionFinishCaptcha,
		CaptchaRequest:    captcha,
	})
}

func handleFinishCaptcha(c *gin.Context, request UserLoginRequest) {
	if request.CaptchaResponse == nil {
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("handleFinishCaptcha", fmt.Errorf("Captcha response not set"), define.LangKeyGeneralInvalidRequest),
			),
		})
		return
	}

	status, ctx := general.ConsumeCaptchaTransaction(request.CaptchaResponse)
	switch status {
	case general.CaptchaConsumeStatusRetry:
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.SuccResponseInfo(),
			NextAction:        NextActionNeedReCaptcha,
		})
		return
	case general.CaptchaConsumeStatusExpired, general.CaptchaConsumeStatusFailed:
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.SuccResponseInfo(),
			NextAction:        NextActionNeedReLogin,
		})
		return
	}

	tran, useCached, generalErr := general.OpenNewSMSTransaction(
		ctx.(UserLoginRequest).AccountPhone,
		nil,
	)
	if generalErr != nil {
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("handleFinishCaptcha")),
		})
		return
	}
	if !useCached {
		if err := utils.SendSMSVerifyCode(tran.AccountPhone(), tran.VerifyCode(), DefaultSmsExpireInMinutes); err != nil {
			general.DiscardSMSTransaction(tran.AccountPhone())
			c.JSON(http.StatusOK, UserLoginResponse{
				BasicResponseInfo: general.FromGeneralError(
					define.NewGeneralError("handleFinishCaptcha", err, define.LangKeyLoginSmsSendFailErr),
				),
			})
			return
		}
	}
	c.JSON(http.StatusOK, UserLoginResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		NextAction:        NextActionReceiveSMSCode,
	})
}

func handleSubmitSMSCode(c *gin.Context, request UserLoginRequest) {
	db := environment.DB.Database()
	userHandle := environment.DB.UserHandle()

	status, _ := general.ConsumeSMSTransaction(request.AccountPhone, request.SMSVerifyCode)
	switch status {
	case general.SmsConsumeStatusExpired:
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("handleSubmitSMSCode", fmt.Errorf("SMS transaction is expired"), define.LangKeyLoginSmsTranExpiredErr),
			),
		})
		return
	case general.SmsConsumeStatusMismatch:
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.SuccResponseInfo(),
			NextAction:        NextActionNeedReSubmit,
		})
		return
	}

	user, found, generalErr := userHandle.QueryUser(db, handle.QueryUserActionSearchByAccountPhone, request.AccountPhone)
	if generalErr != nil {
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("handleSubmitSMSCode")),
		})
		return
	}
	if found {
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.SuccResponseInfo(),
			NextAction:        NextActionFinishLogin,
			UserIdentity:      user.UserIdentity,
			LoginToken:        user.SessionInfo.LoginToken,
		})
		return
	}

	userIdentity, loginToken, generalErr := userHandle.CreateUser(db, request.AccountPhone)
	if generalErr != nil {
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("handleSubmitSMSCode")),
		})
		return
	}
	c.JSON(http.StatusOK, UserLoginResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		NextAction:        NextActionConfigProfile,
		UserIdentity:      userIdentity,
		LoginToken:        loginToken,
	})
}
