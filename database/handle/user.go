package handle

import (
	"errors"
	"fmt"

	define "github.com/Happy2018new/evercare-journey-backed/database/define"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const ExtendSessionDurationDefault = 30 * 24 * 60 * 60

const (
	QueryUserActionSearchByUniqueID uint8 = iota
	QueryUserActionSearchByUserIdentity
	QueryUserActionSearchByAccountName
	QueryUserActionSearchByAccountPhone
)

const (
	UpdateUserLockFlagLockData uint8 = 1 << iota
	UpdateUserLockFlagLockSession
	UpdateUserLockFlagLockProfile
)

const (
	ValidateSessionStatusValidSession uint8 = iota
	ValidateSessionStatusInvalidSalt
	ValidateSessionStatusSaltNotSafe
	ValidateSessionStatusUserNotFound
	ValidateSessionStatusTokenInvalid
	ValidateSessionStatusTokenExpired
	ValidateSessionStatusUnknownError = 255
)

type UserHandle struct{}

func NewUserHandle() *UserHandle {
	return new(UserHandle)
}

func (u *UserHandle) CreateUser(tx *gorm.DB, accountPhone string) (user define.UserData, generalErr *define.GeneralError) {
	user = define.MakeNewUser(accountPhone, define.UserPermissionDefault)

	err := tx.Transaction(func(tx *gorm.DB) error {
		if result := tx.Create(&user); result.Error != nil {
			return result.Error
		}
		if err := u.UpdateLoginToken(tx, user.UserIdentity, user.SessionInfo.LoginToken); err != nil {
			return err
		}
		if err := u.ExtendSession(tx, user.UserIdentity); err != nil {
			return err
		}
		return nil
	})
	if err == nil {
		return user, nil
	}
	if generalErr, ok := err.(*define.GeneralError); ok {
		return define.UserData{}, generalErr.AppendSource("CreateUser")
	}
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		return define.UserData{}, define.NewGeneralError(err, "CreateUser", "创建用户时发生未知错误")
	}

	return define.UserData{}, define.NewGeneralError(
		fmt.Errorf("Target phone number is already used"),
		"CreateUser",
		"目标手机号已被注册",
	)
}

func (u *UserHandle) QueryUser(tx *gorm.DB, action uint8, keyword any) (user define.UserData, found bool, generalErr *define.GeneralError) {
	tx = tx.
		Preload("ProfileData").
		Preload("SessionInfo")

	switch action {
	case QueryUserActionSearchByUniqueID:
		tx = tx.Where("user_unique_id = ?", keyword)
	case QueryUserActionSearchByUserIdentity:
		tx = tx.Where("user_identity = ?", keyword)
	case QueryUserActionSearchByAccountName:
		tx = tx.Where("account_name = ?", keyword)
	case QueryUserActionSearchByAccountPhone:
		tx = tx.Where("account_phone = ?", keyword)
	default:
		return user, false, define.NewGeneralError(
			fmt.Errorf("unsupported action: %d", action),
			"QueryUser",
			"查询用户时发生未知错误",
		)
	}

	result := tx.First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return user, false, nil
	}
	if result.Error != nil {
		return user, false, define.NewGeneralError(
			fmt.Errorf("action = %d, keyword = %v, err = %w", action, keyword, result.Error),
			"QueryUser",
			"查询用户时发生未知错误",
		)
	}

	return user, true, nil
}

func (u *UserHandle) UpdateUser(
	tx *gorm.DB,
	queryAction uint8,
	queryKeyWord any,
	lockFlags uint8,
	userUpdater func(tx *gorm.DB, user *define.UserData) *define.GeneralError,
) *define.GeneralError {
	err := tx.Transaction(func(tx *gorm.DB) error {
		subTx := tx
		if lockFlags&UpdateUserLockFlagLockData != 0 {
			subTx = tx.Clauses(clause.Locking{Strength: "UPDATE"})
		}

		user, found, generalErr := u.QueryUser(subTx, queryAction, queryKeyWord)
		if generalErr != nil {
			return generalErr
		}
		if !found {
			return define.NewGeneralError(fmt.Errorf("Target user not found"), "UpdateUser", "目标用户不存在")
		}

		if lockFlags&UpdateUserLockFlagLockSession != 0 {
			result := tx.
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("user_unique_id = ?", user.UserUniqueID).
				First(&user.SessionInfo)
			if result.Error != nil {
				return define.NewGeneralError(result.Error, "UpdateUser", "更新用户信息时锁定会话信息失败")
			}
		}
		if lockFlags&UpdateUserLockFlagLockProfile != 0 {
			result := tx.
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("user_unique_id = ?", user.UserUniqueID).
				First(&user.ProfileData)
			if result.Error != nil {
				return define.NewGeneralError(result.Error, "UpdateUser", "更新用户信息时锁定用户资料失败")
			}
		}

		generalErr = userUpdater(tx, &user)
		if generalErr != nil {
			return generalErr
		}
		return nil
	})

	if err != nil {
		if generalErr, ok := err.(*define.GeneralError); ok {
			return generalErr
		}
		return define.NewGeneralError(err, "UpdateUser", "更新用户信息时发生未知错误")
	}

	return nil
}

func (u *UserHandle) UpdateLoginToken(tx *gorm.DB, userIdentity string, newToken string) *define.GeneralError {
	generalErr := u.UpdateUser(
		tx,
		QueryUserActionSearchByUserIdentity,
		userIdentity,
		UpdateUserLockFlagLockSession,
		func(tx *gorm.DB, user *define.UserData) *define.GeneralError {
			result := tx.
				Model(&define.UserSession{}).
				Where("user_unique_id = ?", user.UserUniqueID).
				UpdateColumn("login_token", newToken)
			if result.Error != nil {
				return define.NewGeneralError(result.Error, "", "更新登录令牌时发生未知错误")
			}
			if result.RowsAffected == 0 {
				return define.NewGeneralError(fmt.Errorf("session not found"), "", "用户会话未找到")
			}
			return nil
		},
	)
	if generalErr != nil {
		return generalErr.AppendSource("UpdateLoginToken")
	}
	return nil
}

func (u *UserHandle) ExtendSession(tx *gorm.DB, userIdentity string) *define.GeneralError {
	generalErr := u.UpdateUser(
		tx,
		QueryUserActionSearchByUserIdentity,
		userIdentity,
		UpdateUserLockFlagLockSession,
		func(tx *gorm.DB, user *define.UserData) *define.GeneralError {
			result := tx.
				Model(&define.UserSession{}).
				Where("user_unique_id = ?", user.UserUniqueID).
				UpdateColumn(
					"expire_unix_time",
					gorm.Expr("expire_unix_time + ?", ExtendSessionDurationDefault),
				)
			if result.Error != nil {
				return define.NewGeneralError(result.Error, "", "更新会话过期时间时发生未知错误")
			}
			if result.RowsAffected == 0 {
				return define.NewGeneralError(fmt.Errorf("session not found"), "", "用户会话未找到")
			}
			return nil
		},
	)
	if generalErr != nil {
		return generalErr.AppendSource("ExtendSession")
	}
	return nil
}
