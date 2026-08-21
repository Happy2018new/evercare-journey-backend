package profile

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/database/handle"
	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/Happy2018new/evercare-journey-backend/service/auth"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/Happy2018new/evercare-journey-backend/utils"
	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DefaultAvatarImageMaxBytes = 5 * 1024 * 1024
	DefaultAvatarImageMaxSize  = 256
)

func handleAvatarUpload(c *gin.Context, request AvatarUploadRequest) {
	reader := io.LimitReader(
		brotli.NewReader(bytes.NewReader(request.ImageData)),
		DefaultAvatarImageMaxBytes+1,
	)
	data, err := io.ReadAll(reader)
	if err != nil {
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(err, "HandleAvatarUpload", "解压头像数据失败"),
			),
		})
		return
	}
	if len(data) == DefaultAvatarImageMaxBytes+1 {
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(
					fmt.Errorf("Uploaded avatar exceeds %d bytes after decompress", DefaultAvatarImageMaxBytes),
					"HandleAvatarUpload",
					fmt.Sprintf("上传的头像图片不得超过 %d MB", DefaultAvatarImageMaxBytes/1024/1024),
				),
			),
		})
		return
	}

	source, err := utils.ImageFromBytes(data)
	if err != nil {
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(err, "HandleAvatarUpload", "头像图片格式无效"),
			),
		})
		return
	}
	pngData, err := utils.ImageToPNG(utils.ResizeImage(source, DefaultAvatarImageMaxSize))
	if err != nil {
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(err, "HandleAvatarUpload", "处理头像图片失败"),
			),
		})
		return
	}

	avatarItemID := uuid.NewString()
	if err = environment.DB.ResourceHandle().SaveResource(handle.ResourceTypeUserAvatar, avatarItemID, pngData); err != nil {
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(err, "HandleAvatarUpload", "保存头像图片失败"),
			),
		})
		return
	}

	generalErr := environment.DB.UserHandle().UpdateUser(
		environment.DB.Database(),
		handle.QueryUserActionSearchByUserIdentity,
		request.UserIdentity,
		handle.UpdateUserLockFlagLockData,
		func(tx *gorm.DB, user *define.UserData) *define.GeneralError {
			result := tx.Model(&define.UserProfile{}).
				Where("user_unique_id = ?", user.UserUniqueID).
				UpdateColumn("avatar_item_id", avatarItemID)
			if result.Error != nil {
				return define.NewGeneralError(result.Error, "", "更新头像信息失败")
			}
			if result.RowsAffected == 0 {
				return define.NewGeneralError(fmt.Errorf("Profile data not found"), "", "用户资料未找到")
			}
			return nil
		},
	)
	if generalErr != nil {
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("HandleAvatarUpload")),
		})
		return
	}

	c.JSON(http.StatusOK, AvatarUploadResponse{BasicResponseInfo: general.SuccResponseInfo()})
	_, _, _ = auth.LoadUser(request.UserIdentity, true)
}
