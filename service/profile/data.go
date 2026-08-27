package profile

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/database/handle"
	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/Happy2018new/evercare-journey-backend/service/auth"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func handleQueryProfileData(c *gin.Context, request ProfileDataRequest) {
	user, found, generalErr := auth.LoadUser(request.UserIdentity, false)
	if generalErr != nil {
		c.JSON(http.StatusOK, ProfileDataResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("HandleProfileData")),
		})
		return
	}
	if !found {
		c.JSON(http.StatusOK, ProfileDataResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("handleQueryProfileData", fmt.Errorf("Should never happened"), define.LangKeyGeneralInvalidSession),
			),
		})
		return
	}
	if user.ProfileData.UserUniqueID != user.UserUniqueID {
		c.JSON(http.StatusOK, ProfileDataResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("handleQueryProfileData", fmt.Errorf("user profile row is missing"), define.LangKeyUserQueryUnknownErr),
			),
		})
		return
	}
	c.JSON(http.StatusOK, ProfileDataResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		Name:              user.AccountName,
		Gender:            user.ProfileData.Gender,
		Age:               user.ProfileData.Age,
	})
}

func handleUpdateProfileExtraData(c *gin.Context, request ProfileDataRequest) {
	switch request.Gender {
	case define.UserGenderNotSet:
	case define.UserGenderMan, define.UserGenderWoman:
	default:
		c.JSON(http.StatusOK, ProfileDataResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(
					"handleUpdateProfileExtraData",
					fmt.Errorf("Invalid user gender %d", request.Gender),
					define.LangKeyProfileGenderInvalid,
				),
			),
		})
		return
	}
	switch {
	case request.Age == define.UserAgeNotSet:
	case define.UserMinAge <= request.Age && request.Age <= define.UserMaxAge:
	default:
		c.JSON(http.StatusOK, ProfileDataResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(
					"handleUpdateProfileExtraData",
					fmt.Errorf("Invalid user age %d", request.Age),
					define.LangKeyProfileAgeInvalid,
				),
			),
		})
		return
	}

	generalErr := environment.DB.UserHandle().UpdateUser(
		environment.DB.Database(),
		handle.QueryUserActionSearchByUserIdentity,
		request.UserIdentity,
		handle.UpdateUserLockFlagLockProfile,
		func(tx *gorm.DB, user *define.UserData) *define.GeneralError {
			// MySQL may report zero affected rows for a valid no-op update.
			// Treat unchanged profile data as success instead of surfacing a
			// misleading persistence error to the client.
			if user.ProfileData.UserUniqueID == user.UserUniqueID &&
				user.ProfileData.Gender == request.Gender && user.ProfileData.Age == request.Age {
				return nil
			}
			result := tx.Model(&define.UserProfile{}).
				Where("user_unique_id = ?", user.UserUniqueID).
				Updates(map[string]any{
					"gender": request.Gender,
					"age":    request.Age,
				})
			if result.Error != nil {
				return define.NewGeneralError("", result.Error, define.LangKeyUserUpdateProfileFailErr)
			}
			if result.RowsAffected != 1 {
				return define.NewGeneralError("", fmt.Errorf("user profile row was not updated"), define.LangKeyUserUpdateProfileFailErr)
			}
			return nil
		},
	)
	if generalErr != nil {
		c.JSON(http.StatusOK, ProfileDataResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("handleUpdateProfileExtraData")),
		})
		return
	}

	c.JSON(http.StatusOK, ProfileDataResponse{BasicResponseInfo: general.SuccResponseInfo()})
	auth.InvalidateUserCache(request.UserIdentity)
}

func handleUpdateProfileName(c *gin.Context, request ProfileDataRequest) {
	request.Name = strings.TrimSpace(request.Name)
	if valid, reason := define.IsValidAccountName(request.Name); !valid {
		c.JSON(http.StatusOK, ProfileDataResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("handleUpdateProfileName", fmt.Errorf("Invalid account name %s; reason = %s", request.Name, reason), reason),
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
			if user.AccountName == request.Name {
				return nil
			}
			result := tx.Model(&define.UserData{}).
				Where("user_unique_id = ?", user.UserUniqueID).
				UpdateColumn("account_name", request.Name)
			if result.Error != nil {
				if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
					return define.NewGeneralError("", result.Error, define.LangKeyUserUpdateNameUsedErr)
				}
				return define.NewGeneralError("", result.Error, define.LangKeyUserUpdateProfileFailErr)
			}
			if result.RowsAffected != 1 {
				return define.NewGeneralError("", fmt.Errorf("user account row was not updated"), define.LangKeyUserUpdateProfileFailErr)
			}
			return nil
		},
	)
	if generalErr != nil {
		c.JSON(http.StatusOK, ProfileDataResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("handleUpdateProfileName")),
		})
		return
	}

	c.JSON(http.StatusOK, ProfileDataResponse{BasicResponseInfo: general.SuccResponseInfo()})
	auth.InvalidateUserCache(request.UserIdentity)
}
