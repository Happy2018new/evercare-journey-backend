package general

import "github.com/Happy2018new/evercare-journey-backend/database/define"

type BasicSessionInfo struct {
	UserIdentity   string `json:"user_identity"`
	RandomSalt     string `json:"random_salt"`
	EncryptedToken string `json:"encrypted_token"`
}

type BasicResponseInfo struct {
	SuccessStates  bool               `json:"success_states"`
	DebugErrorInfo string             `json:"debug_error_info,omitempty"`
	PublicErrorMsg *define.LangFormat `json:"public_error_msg,omitempty"`
	ExtraErrorData any                `json:"extra_error_data,omitempty"`
}

func SuccResponseInfo() BasicResponseInfo {
	return BasicResponseInfo{SuccessStates: true}
}

func FromGeneralError(generalErr *define.GeneralError) BasicResponseInfo {
	if generalErr == nil {
		return SuccResponseInfo()
	}
	return BasicResponseInfo{
		SuccessStates:  false,
		DebugErrorInfo: generalErr.Error(),
		PublicErrorMsg: generalErr.GetPublicMsg(),
		ExtraErrorData: generalErr.GetExtraData(),
	}
}
