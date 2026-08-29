package security

import (
	"fmt"
	"net/http"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/Happy2018new/evercare-journey-backend/service/auth"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/gin-gonic/gin"
)

func loadUser(session general.BasicSessionInfo, source string) (*define.UserData, *define.GeneralError) {
	status, ge := auth.ValidateSession(session)
	if ge != nil { return nil, ge.AppendSource(source) }
	if status != auth.ValidateSessionStatusValidSession { return nil, define.NewGeneralError(source, fmt.Errorf("invalid session"), define.LangKeyGeneralInvalidSession) }
	user, found, ge := auth.LoadUser(session.UserIdentity, false)
	if ge != nil { return nil, ge.AppendSource(source) }
	if !found || user == nil { return nil, define.NewGeneralError(source, fmt.Errorf("user not found"), define.LangKeyGeneralInvalidSession) }
	return user, nil
}

func settingData(setting define.SafetySetting) SafetySettingData {
	return SafetySettingData{EmergencyLocationShare: setting.EmergencyLocationShare, SOSConfirmation: setting.SOSConfirmation}
}

func respond(c *gin.Context, response any, ge *define.GeneralError) {
	if ge == nil { c.JSON(http.StatusOK, response); return }
	info := general.FromGeneralError(ge)
	switch value := response.(type) {
	case QuerySettingResponse: value.BasicResponseInfo = info; c.JSON(http.StatusOK, value)
	case UpdateSettingResponse: value.BasicResponseInfo = info; c.JSON(http.StatusOK, value)
	}
}

func HandleQuerySetting(c *gin.Context) {
	const source = "HandleQuerySafetySetting"
	var request QuerySettingRequest
	if err := c.ShouldBind(&request); err != nil { respond(c, QuerySettingResponse{}, define.NewGeneralError(source, err, define.LangKeySafetyRequestBodyInvalid)); return }
	user, ge := loadUser(request.BasicSessionInfo, source); if ge != nil { respond(c, QuerySettingResponse{}, ge); return }
	setting, ge := environment.DB.SafetyHandle().QuerySetting(environment.DB.Database(), user.UserUniqueID); if ge != nil { respond(c, QuerySettingResponse{}, ge); return }
	c.JSON(http.StatusOK, QuerySettingResponse{BasicResponseInfo: general.SuccResponseInfo(), Setting: settingData(setting)})
}

func HandleUpdateSetting(c *gin.Context) {
	const source = "HandleUpdateSafetySetting"
	var request UpdateSettingRequest
	if err := c.ShouldBind(&request); err != nil { respond(c, UpdateSettingResponse{}, define.NewGeneralError(source, err, define.LangKeySafetyRequestBodyInvalid)); return }
	if request.EmergencyLocationShare == nil && request.SOSConfirmation == nil { respond(c, UpdateSettingResponse{}, define.NewGeneralError(source, fmt.Errorf("no setting provided"), define.LangKeySafetySettingInvalid)); return }
	user, ge := loadUser(request.BasicSessionInfo, source); if ge != nil { respond(c, UpdateSettingResponse{}, ge); return }
	current, ge := environment.DB.SafetyHandle().QuerySetting(environment.DB.Database(), user.UserUniqueID); if ge != nil { respond(c, UpdateSettingResponse{}, ge); return }
	locationShare, confirmation := current.EmergencyLocationShare, current.SOSConfirmation
	if request.EmergencyLocationShare != nil { locationShare = *request.EmergencyLocationShare }
	if request.SOSConfirmation != nil { confirmation = *request.SOSConfirmation }
	setting, ge := environment.DB.SafetyHandle().UpdateSetting(environment.DB.Database(), user.UserUniqueID, locationShare, confirmation); if ge != nil { respond(c, UpdateSettingResponse{}, ge); return }
	c.JSON(http.StatusOK, UpdateSettingResponse{BasicResponseInfo: general.SuccResponseInfo(), Setting: settingData(setting)})
}
