package auth

import (
	"fmt"
	"net/http"

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
					fmt.Errorf("Provided phone number must have 11 characters"),
					"handleLoginRequest",
					"手机号的长度必须为 11 位",
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
				define.NewGeneralError(fmt.Errorf("Captcha response not set"), "handleFinishCaptcha", "无效请求"),
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

	tran, generalErr := general.OpenNewSMSTransaction(
		ctx.(UserLoginRequest).AccountPhone,
		nil,
	)
	if generalErr != nil {
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("handleFinishCaptcha")),
		})
		return
	}
	if err := utils.SendSMSVerifyCode(tran.AccountPhone(), tran.VerifyCode(), DefaultSmsExpireInMinutes); err != nil {
		general.DiscardSMSTransaction(tran.AccountPhone())
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(err, "handleFinishCaptcha", "发送短信验证码时出现未知错误"),
			),
		})
		return
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
				define.NewGeneralError(fmt.Errorf("SMS transaction is expired"), "handleSubmitSMSCode", "短信验证码已过期，请重新登录"),
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
