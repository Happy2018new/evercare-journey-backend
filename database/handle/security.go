package handle

import (
	"fmt"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"gorm.io/gorm"
)

type SafetyHandle struct{}

func NewSafetyHandle() *SafetyHandle { return new(SafetyHandle) }

func (h *SafetyHandle) QuerySetting(tx *gorm.DB, userID uint32) (define.SafetySetting, *define.GeneralError) {
	var setting define.SafetySetting
	result := tx.Where("user_unique_id = ?", userID).First(&setting)
	if result.Error == nil {
		return setting, nil
	}
	if result.Error != gorm.ErrRecordNotFound {
		return setting, define.NewGeneralError("QuerySafetySetting", result.Error, define.LangKeySafetySettingQueryUnknown)
	}
	setting = define.SafetySetting{UserUniqueID: userID, SOSConfirmation: true}
	if result = tx.Create(&setting); result.Error != nil {
		return setting, define.NewGeneralError("CreateSafetySetting", result.Error, define.LangKeySafetySettingUpdateUnknown)
	}
	return setting, nil
}

func (h *SafetyHandle) UpdateSetting(tx *gorm.DB, userID uint32, locationShare, sosConfirmation bool) (define.SafetySetting, *define.GeneralError) {
	setting := define.SafetySetting{UserUniqueID: userID, EmergencyLocationShare: locationShare, SOSConfirmation: sosConfirmation}
	result := tx.Save(&setting)
	if result.Error != nil {
		return setting, define.NewGeneralError("UpdateSafetySetting", fmt.Errorf("%w", result.Error), define.LangKeySafetySettingUpdateUnknown)
	}
	return setting, nil
}
