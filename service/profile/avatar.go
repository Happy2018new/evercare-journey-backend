package profile

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/database/handle"
	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/Happy2018new/evercare-journey-backend/service/auth"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/Happy2018new/evercare-journey-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DefaultAvatarImageMaxBytes = 5 * 1024 * 1024
	DefaultAvatarImageMaxSize  = 256
)

func handleAvatarQuery(c *gin.Context, request AvatarQueryRequest) {
	var result []byte
	var err error

	user, found, generalErr := environment.DB.UserHandle().QueryUser(
		environment.DB.Database(),
		handle.QueryUserActionSearchByUserIdentity,
		request.UserIdentity,
	)
	if generalErr != nil {
		c.JSON(http.StatusOK, AvatarQueryResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("handleAvatarQuery")),
		})
		return
	}

	pngData, found := environment.DB.ResourceHandle().LoadResource(
		handle.ResourceTypeUserAvatar,
		user.ProfileData.AvatarItemID,
	)
	if !found {
		c.JSON(http.StatusOK, AvatarQueryResponse{
			BasicResponseInfo: general.SuccResponseInfo(),
			AvatarSet:         false,
		})
		return
	}
	rawCheckSum := sha512.Sum512(pngData)
	ansCheckSum := hex.EncodeToString(rawCheckSum[:])

	if request.QueryAction == AvatarQueryActionGetImageData {
		result, err = utils.CompressBrotli(pngData)
		if err != nil {
			c.JSON(http.StatusOK, AvatarQueryResponse{
				BasicResponseInfo: general.FromGeneralError(
					define.NewGeneralError("handleAvatarGetImage", err, define.LangKeyAvatarZipFailErr),
				),
			})
			return
		}
	}

	c.JSON(http.StatusOK, AvatarQueryResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		AvatarSet:         true,
		Checksum:          ansCheckSum,
		ImageData:         result,
	})
}

func handleAvatarUpload(c *gin.Context, request AvatarUploadRequest) {
	rawData, exceeded, err := utils.DecompressBrotli(request.ImageData, DefaultAvatarImageMaxBytes)
	if err != nil {
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("handleAvatarUpload", err, define.LangKeyAvatarUnzipFailErr),
			),
		})
		return
	}
	if exceeded {
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(
					"handleAvatarUpload",
					fmt.Errorf("Uploaded avatar exceeds %d bytes after decompress", DefaultAvatarImageMaxBytes),
					define.LangKeyAvatarReachMaxSizeErr,
					fmt.Sprintf("%d", DefaultAvatarImageMaxBytes/1024/1024),
				),
			),
		})
		return
	}

	decoded, err := utils.ImageFromBytes(rawData)
	if err != nil {
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("handleAvatarUpload", err, define.LangKeyAvatarInvalidData),
			),
		})
		return
	}
	pngData, err := utils.ImageToPNG(utils.ResizeImage(decoded, DefaultAvatarImageMaxSize))
	if err != nil {
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("handleAvatarUpload", err, define.LangKeyAvatarConvertFailErr),
			),
		})
		return
	}

	user, found, generalErr := auth.LoadUser(request.UserIdentity, false)
	if generalErr != nil {
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("handleAvatarUpload")),
		})
		return
	}
	if !found {
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("handleAvatarUpload", fmt.Errorf("Should never happened (mark 0)"), define.LangKeyGeneralUnknownErr),
			),
		})
		return
	}
	if len(user.ProfileData.AvatarItemID) > 0 {
		err = environment.DB.ResourceHandle().SaveResource(handle.ResourceTypeUserAvatar, user.ProfileData.AvatarItemID, pngData)
		if err == nil {
			c.JSON(http.StatusOK, AvatarUploadResponse{BasicResponseInfo: general.SuccResponseInfo()})
		} else {
			c.JSON(http.StatusOK, AvatarUploadResponse{
				BasicResponseInfo: general.FromGeneralError(
					define.NewGeneralError("handleAvatarUpload", err, define.LangKeyAvatarSaveFailErr),
				),
			})
		}
		return
	}

	avatarItemID := uuid.NewString()
	generalErr = environment.DB.UserHandle().UpdateUser(
		environment.DB.Database(),
		handle.QueryUserActionSearchByUserIdentity,
		request.UserIdentity,
		handle.UpdateUserLockFlagLockProfile,
		func(tx *gorm.DB, user *define.UserData) *define.GeneralError {
			if len(user.ProfileData.AvatarItemID) > 0 {
				return define.NewGeneralError("", fmt.Errorf("Should never happened (mark 1)"), define.LangKeyGeneralUnknownErr)
			}
			if err = environment.DB.ResourceHandle().SaveResource(handle.ResourceTypeUserAvatar, avatarItemID, pngData); err != nil {
				return define.NewGeneralError("", err, define.LangKeyAvatarSaveFailErr)
			}

			result := tx.Model(&define.UserProfile{}).
				Where("user_unique_id = ?", user.UserUniqueID).
				UpdateColumn("avatar_item_id", avatarItemID)
			if result.Error != nil {
				return define.NewGeneralError("", result.Error, define.LangKeyAvatarUpdateFailErr)
			}

			return nil
		},
	)
	if generalErr != nil {
		_ = environment.DB.ResourceHandle().DeleteResource(handle.ResourceTypeUserAvatar, avatarItemID)
		c.JSON(http.StatusOK, AvatarUploadResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("handleAvatarUpload")),
		})
		return
	}

	c.JSON(http.StatusOK, AvatarUploadResponse{BasicResponseInfo: general.SuccResponseInfo()})
	_, _, _ = auth.LoadUser(request.UserIdentity, true)
}
