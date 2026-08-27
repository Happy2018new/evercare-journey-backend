package auth

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/database/handle"
	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/Happy2018new/evercare-journey-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DefaultAccountPhoneLength = 11
	DefaultSmsExpireInMinutes = "5"
)

func validateAccountPhone(source string, phone string) *define.GeneralError {
	if len(phone) != DefaultAccountPhoneLength {
		return define.NewGeneralError(
			source,
			fmt.Errorf("provided phone number must have %d characters", DefaultAccountPhoneLength),
			define.LangKeyLoginPhoneLengthErr,
			fmt.Sprintf("%d", DefaultAccountPhoneLength),
		)
	}
	if _, err := strconv.ParseUint(phone, 10, 64); err != nil {
		return define.NewGeneralError(
			source,
			fmt.Errorf("invalid phone number %s", phone),
			define.LangKeyLoginPhoneInvalidErr,
		)
	}
	// The client and SMS provider use the mainland China mobile-number flow.
	// Checking the prefix here prevents accounts such as 00000000000 from
	// being created by callers that bypass the UI validation.
	if phone[0] != '1' || phone[1] < '3' || phone[1] > '9' {
		return define.NewGeneralError(
			source,
			fmt.Errorf("phone number has an unsupported mobile prefix"),
			define.LangKeyLoginPhoneInvalidErr,
		)
	}
	return nil
}

func validateSMSCode(source string, value string) *define.GeneralError {
	value = strings.TrimSpace(value)
	if len(value) != 6 {
		return define.NewGeneralError(source, fmt.Errorf("SMS verify code must have 6 characters"), define.LangKeyLoginSmsCodeInvalid)
	}
	if _, err := strconv.ParseUint(value, 10, 32); err != nil {
		return define.NewGeneralError(source, fmt.Errorf("SMS verify code must contain only digits"), define.LangKeyLoginSmsCodeInvalid)
	}
	return nil
}

func handleLoginRequest(c *gin.Context, request UserLoginRequest) {
	if generalErr := validateAccountPhone("handleLoginRequest", request.AccountPhone); generalErr != nil {
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr),
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
				define.NewGeneralError("handleFinishCaptcha", fmt.Errorf("captcha response not set"), define.LangKeyLoginCaptchaInvalid),
			),
		})
		return
	}
	transactionUUID, err := uuid.Parse(request.CaptchaResponse.TransactionUUID)
	if err != nil || transactionUUID == uuid.Nil {
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("handleFinishCaptcha", fmt.Errorf("captcha transaction identity is invalid"), define.LangKeyLoginCaptchaInvalid),
			),
		})
		return
	}
	request.CaptchaResponse.TransactionUUID = transactionUUID.String()

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

	loginRequest, ok := ctx.(UserLoginRequest)
	if !ok {
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("handleFinishCaptcha", fmt.Errorf("captcha context is invalid"), define.LangKeyGeneralUnknownErr),
			),
		})
		return
	}
	if generalErr := validateAccountPhone("handleFinishCaptcha", loginRequest.AccountPhone); generalErr != nil {
		c.JSON(http.StatusOK, UserLoginResponse{BasicResponseInfo: general.FromGeneralError(generalErr)})
		return
	}
	tran, useCached, generalErr := general.OpenNewSMSTransaction(
		loginRequest.AccountPhone,
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
	request.AccountPhone = strings.TrimSpace(request.AccountPhone)
	request.SMSVerifyCode = strings.TrimSpace(request.SMSVerifyCode)
	if generalErr := validateAccountPhone("handleSubmitSMSCode", request.AccountPhone); generalErr != nil {
		c.JSON(http.StatusOK, UserLoginResponse{BasicResponseInfo: general.FromGeneralError(generalErr)})
		return
	}
	if generalErr := validateSMSCode("handleSubmitSMSCode", request.SMSVerifyCode); generalErr != nil {
		c.JSON(http.StatusOK, UserLoginResponse{BasicResponseInfo: general.FromGeneralError(generalErr)})
		return
	}
	db := environment.DB.Database()
	userHandle := environment.DB.UserHandle()

	status, _ := general.ConsumeSMSTransaction(request.AccountPhone, request.SMSVerifyCode)
	switch status {
	case general.SmsConsumeStatusSuccess:
		// Continue with account lookup and session issuance below.
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
	case general.SmsConsumeStatusTooManyAttempts:
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("handleSubmitSMSCode", fmt.Errorf("too many SMS verification attempts"), define.LangKeyLoginSmsTooManyAttempts),
			),
		})
		return
	default:
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("handleSubmitSMSCode", fmt.Errorf("unknown SMS transaction status %d", status), define.LangKeyLoginSmsTranExpiredErr),
			),
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
		newLoginToken := uuid.NewString()
		generalErr = userHandle.UpdateUser(
			db,
			handle.QueryUserActionSearchByAccountPhone,
			request.AccountPhone,
			handle.UpdateUserLockFlagLockSession,
			func(tx *gorm.DB, currentUser *define.UserData) *define.GeneralError {
				result := tx.Model(&define.UserSession{}).
					Where("user_unique_id = ?", currentUser.UserUniqueID).
					Updates(map[string]any{
						"login_token":      newLoginToken,
						"expire_unix_time": time.Now().Unix() + define.ExtendSessionDurationDefault,
					})
				if result.Error != nil {
					return define.NewGeneralError("handleSubmitSMSCode", result.Error, define.LangKeyLoginSessionRefreshErr)
				}
				if result.RowsAffected != 1 {
					return define.NewGeneralError("handleSubmitSMSCode", fmt.Errorf("user session row was not updated"), define.LangKeyLoginSessionRefreshErr)
				}
				return nil
			},
		)
		if generalErr != nil {
			c.JSON(http.StatusOK, UserLoginResponse{
				BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("handleSubmitSMSCode")),
			})
			return
		}
		// The cached user still contains the old token/expiry. Remove it before
		// returning the freshly issued token so the next request cannot validate
		// against stale session data.
		cachedSessionInfo.Delete(user.UserIdentity)
		c.JSON(http.StatusOK, UserLoginResponse{
			BasicResponseInfo: general.SuccResponseInfo(),
			NextAction:        NextActionFinishLogin,
			UserIdentity:      user.UserIdentity,
			LoginToken:        newLoginToken,
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
