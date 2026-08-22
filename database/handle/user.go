package handle

import (
	"errors"
	"fmt"

	define "github.com/Happy2018new/evercare-journey-backend/database/define"
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

func (u *UserHandle) CreateUser(tx *gorm.DB, accountPhone string) (userIdentity string, loginToken string, generalErr *define.GeneralError) {
	user := define.MakeNewUser(accountPhone, define.UserPermissionDefault)

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
		return user.UserIdentity, user.SessionInfo.LoginToken, nil
	}
	if generalErr, ok := err.(*define.GeneralError); ok {
		return "", "", generalErr.AppendSource("CreateUser")
	}
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		return "", "", define.NewGeneralError("CreateUser", err, define.LangKeyUserCreateUnknownErr)
	}

	return "", "", define.NewGeneralError(
		"CreateUser",
		fmt.Errorf("Target phone number is already used"),
		define.LangKeyUserCreatePhoneUsedErr,
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
			"QueryUser",
			fmt.Errorf("unsupported action %d", action),
			define.LangKeyUserQueryUnknownErr,
		)
	}

	result := tx.First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return user, false, nil
	}
	if result.Error != nil {
		return user, false, define.NewGeneralError(
			"QueryUser",
			fmt.Errorf("action = %d, keyword = %v, err = %w", action, keyword, result.Error),
			define.LangKeyUserQueryUnknownErr,
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
			return define.NewGeneralError("", fmt.Errorf("Target user not found"), define.LangKeyUserQueryNotFoundErr)
		}

		if lockFlags&UpdateUserLockFlagLockSession != 0 {
			result := tx.
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("user_unique_id = ?", user.UserUniqueID).
				First(&user.SessionInfo)
			if result.Error != nil {
				return define.NewGeneralError("", result.Error, define.LangKeyUserUpdateLockSessionFailErr)
			}
		}
		if lockFlags&UpdateUserLockFlagLockProfile != 0 {
			result := tx.
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("user_unique_id = ?", user.UserUniqueID).
				First(&user.ProfileData)
			if result.Error != nil {
				return define.NewGeneralError("", result.Error, define.LangKeyUserUpdateLockProfileFailErr)
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
			return generalErr.AppendSource("UpdateUser")
		}
		return define.NewGeneralError("UpdateUser", err, define.LangKeyUserUpdateUnknownErr)
	}

	return nil
}

func (u *UserHandle) UpdateLoginToken(tx *gorm.DB, userIdentity string, newToken string) *define.GeneralError {
	generalErr := u.UpdateUser(
		tx,
		QueryUserActionSearchByUserIdentity,
		userIdentity,
		0,
		func(tx *gorm.DB, user *define.UserData) *define.GeneralError {
			result := tx.
				Model(&define.UserSession{}).
				Where("user_unique_id = ?", user.UserUniqueID).
				UpdateColumn("login_token", newToken)
			if result.Error != nil {
				return define.NewGeneralError("", result.Error, define.LangKeyUserUpdateLoginTokenErr)
			}
			if result.RowsAffected == 0 {
				return define.NewGeneralError("", fmt.Errorf("User session not found"), define.LangKeyUserSessionNotFoundErr)
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
		0,
		func(tx *gorm.DB, user *define.UserData) *define.GeneralError {
			result := tx.
				Model(&define.UserSession{}).
				Where("user_unique_id = ?", user.UserUniqueID).
				UpdateColumn(
					"expire_unix_time",
					gorm.Expr("expire_unix_time + ?", ExtendSessionDurationDefault),
				)
			if result.Error != nil {
				return define.NewGeneralError("", result.Error, define.LangKeyUserSessionExtendErr)
			}
			if result.RowsAffected == 0 {
				return define.NewGeneralError("", fmt.Errorf("User session not found"), define.LangKeyUserSessionNotFoundErr)
			}
			return nil
		},
	)
	if generalErr != nil {
		return generalErr.AppendSource("ExtendSession")
	}
	return nil
}
