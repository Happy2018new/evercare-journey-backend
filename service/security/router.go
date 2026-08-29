package security

import "github.com/Happy2018new/evercare-journey-backend/service/general"

type SafetySettingData struct {
	EmergencyLocationShare bool `json:"emergency_location_share"`
	SOSConfirmation        bool `json:"sos_confirmation"`
}

type QuerySettingRequest struct{ general.BasicSessionInfo }
type QuerySettingResponse struct {
	general.BasicResponseInfo
	Setting SafetySettingData `json:"setting"`
}

type UpdateSettingRequest struct {
	general.BasicSessionInfo
	EmergencyLocationShare *bool `json:"emergency_location_share"`
	SOSConfirmation        *bool `json:"sos_confirmation"`
}
type UpdateSettingResponse struct {
	general.BasicResponseInfo
	Setting SafetySettingData `json:"setting"`
}
