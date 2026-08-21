package general

import "github.com/Happy2018new/evercare-journey-backend/database/define"

type BasicResponseInfo struct {
	SuccessStates  bool   `json:"success_states"`
	DebugErrorInfo string `json:"debug_error_info,omitempty"`
	PublicErrorMsg string `json:"public_error_msg,omitempty"`
	ExtraErrorData any    `json:"extra_error_data,omitempty"`
}

func SuccResponseInfo() BasicResponseInfo {
	return BasicResponseInfo{SuccessStates: true}
}

func FromGeneralError(generalErr *define.GeneralError) BasicResponseInfo {
	if generalErr == nil {
		return BasicResponseInfo{SuccessStates: true}
	}
	return BasicResponseInfo{
		SuccessStates:  false,
		DebugErrorInfo: generalErr.Error(),
		PublicErrorMsg: generalErr.GetPublicMsg(),
		ExtraErrorData: generalErr.GetExtraData(),
	}
}
