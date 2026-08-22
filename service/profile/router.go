package profile

import (
	"fmt"
	"net/http"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/service/auth"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/gin-gonic/gin"
)

const (
	ProfileDataActionQueryAllData uint8 = iota
	ProfileDataActionUpdateName
	ProfileDataActionUpdateExtra
)

type ProfileDataRequest struct {
	general.BasicSessionInfo
	Action uint8  `json:"action"`
	Name   string `json:"name,omitempty"`
	Gender uint8  `json:"gender,omitempty"`
	Age    uint8  `json:"age,omitempty"`
}

type ProfileDataResponse struct {
	general.BasicResponseInfo
	Name   string `json:"name"`
	Gender uint8  `json:"gender"`
	Age    uint8  `json:"age"`
}

func HandleProfileData(c *gin.Context) {
	var request ProfileDataRequest

	err := c.Bind(&request)
	if err != nil {
		c.JSON(http.StatusOK, ProfileDataResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("HandleProfileData", err, define.LangKeyGeneralInvalidRequest),
			),
		})
		return
	}

	status, generalErr := auth.ValidateSession(request.BasicSessionInfo)
	if generalErr != nil {
		c.JSON(http.StatusOK, ProfileDataResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("HandleProfileData")),
		})
		return
	}
	if status != auth.ValidateSessionStatusValidSession {
		c.JSON(http.StatusOK, ProfileDataResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("HandleProfileData", fmt.Errorf("Failed to validate current session"), define.LangKeyGeneralInvalidSession),
			),
		})
		return
	}

	switch request.Action {
	case ProfileDataActionQueryAllData:
		handleQueryProfileData(c, request)
		return
	case ProfileDataActionUpdateName:
		handleUpdateProfileName(c, request)
		return
	case ProfileDataActionUpdateExtra:
		handleUpdateProfileExtraData(c, request)
		return
	}

	c.JSON(http.StatusOK, ProfileDataResponse{
		BasicResponseInfo: general.FromGeneralError(
			define.NewGeneralError(
				"HandleProfileData",
				fmt.Errorf("Unsupported profile data request action %d", request.Action),
				define.LangKeyGeneralInvalidRequest,
			),
		),
	})
}

const (
	AvatarQueryActionGetChecksum uint8 = iota
	AvatarQueryActionGetData
)

type AvatarQueryRequest struct {
	general.BasicSessionInfo
	QueryAction       uint8  `json:"query_action"`
	QueryUserIdentity string `json:"query_user_identity"`
}

type AvatarQueryResponse struct {
	general.BasicResponseInfo
	AvatarSet bool   `json:"avatar_set"`
	Checksum  string `json:"checksum"`
	ImageData []byte `json:"image_data"`
}

func HandleAvatarQuery(c *gin.Context) {
	var request AvatarQueryRequest

	err := c.Bind(&request)
	if err != nil {
		c.JSON(http.StatusOK, AvatarQueryResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("HandleAvatarQuery", err, define.LangKeyGeneralInvalidRequest),
			),
		})
		return
	}

	status, generalErr := auth.ValidateSession(request.BasicSessionInfo)
	if generalErr != nil {
		c.JSON(http.StatusOK, AvatarQueryResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("HandleAvatarQuery")),
		})
		return
	}
	if status != auth.ValidateSessionStatusValidSession {
		c.JSON(http.StatusOK, AvatarQueryResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("HandleAvatarQuery", fmt.Errorf("Failed to validate current session"), define.LangKeyGeneralInvalidSession),
			),
		})
		return
	}

	handleAvatarQuery(c, request)
}

type AvatarUploadRequest struct {
	general.BasicSessionInfo
	ImageData []byte `json:"image_data"`
}

type AvatarUploadResponse struct {
	general.BasicResponseInfo
}

func HandleAvatarUpload(c *gin.Context) {
	var request AvatarUploadRequest

	err := c.Bind(&request)
	if err != nil {
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("HandleAvatarUpload", err, define.LangKeyGeneralInvalidRequest),
			),
		})
		return
	}

	status, generalErr := auth.ValidateSession(request.BasicSessionInfo)
	if generalErr != nil {
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("HandleAvatarUpload")),
		})
		return
	}
	if status != auth.ValidateSessionStatusValidSession {
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("HandleAvatarUpload", fmt.Errorf("Failed to validate current session"), define.LangKeyGeneralInvalidSession),
			),
		})
		return
	}

	handleAvatarUpload(c, request)
}
