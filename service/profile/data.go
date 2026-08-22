package profile

import (
	"fmt"
	"net/http"

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
					define.LangKeyGeneralInvalidRequest,
				),
			),
		})
		return
	}
	switch {
	case request.Age == define.UserAgeNotSet:
	case request.Age < define.UserMinAge || request.Age > define.UserMaxAge:
	default:
		c.JSON(http.StatusOK, ProfileDataResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(
					"handleUpdateProfileExtraData",
					fmt.Errorf("Invalid user age %d", request.Age),
					define.LangKeyGeneralInvalidRequest,
				),
			),
		})
		return
	}

	generalErr := environment.DB.UserHandle().UpdateUser(
		environment.DB.Database(),
		handle.QueryUserActionSearchByUserIdentity,
		request.UserIdentity,
		0,
		func(tx *gorm.DB, user *define.UserData) *define.GeneralError {
			result := tx.Model(&define.UserProfile{}).
				Where("user_unique_id = ?", user.UserUniqueID).
				Updates(map[string]any{
					"gender": request.Gender,
					"age":    request.Age,
				})
			if result.Error != nil {
				return define.NewGeneralError("", result.Error, define.LangKeyUserUpdateProfileFailErr)
			}
			if result.RowsAffected == 0 {
				return define.NewGeneralError("", fmt.Errorf("Should never happened"), define.LangKeyGeneralInvalidSession)
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
	_, _, _ = auth.LoadUser(request.UserIdentity, true)
}

func handleUpdateProfileName(c *gin.Context, request ProfileDataRequest) {
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
		0,
		func(tx *gorm.DB, user *define.UserData) *define.GeneralError {
			result := tx.Model(&define.UserData{}).
				Where("user_unique_id = ?", user.UserUniqueID).
				UpdateColumn("account_name", request.Name)
			if result.Error != nil {
				return define.NewGeneralError("", result.Error, define.LangKeyUserUpdateProfileFailErr)
			}
			if result.RowsAffected == 0 {
				return define.NewGeneralError("", fmt.Errorf("Should never happened"), define.LangKeyGeneralInvalidSession)
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
	_, _, _ = auth.LoadUser(request.UserIdentity, true)
}
