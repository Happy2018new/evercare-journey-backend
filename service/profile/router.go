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
				define.NewGeneralError(err, "HandleProfileData", "无效请求"),
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
				define.NewGeneralError(fmt.Errorf("Failed to validate current session"), "HandleProfileData", "无效的用户登录状态，请重新启动程序再试"),
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
				fmt.Errorf("Unsupported profile data request action %d", request.Action),
				"HandleProfileData",
				"无效请求",
			),
		),
	})
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
				define.NewGeneralError(err, "HandleAvatarUpload", "无效请求"),
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
				define.NewGeneralError(fmt.Errorf("failed to validate current session"), "HandleAvatarUpload", "无效的用户登录状态，请重新启动程序再试"),
			),
		})
		return
	}

	handleAvatarUpload(c, request)
}
