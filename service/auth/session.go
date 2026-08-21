package auth

import (
	"crypto/sha512"
	"encoding/hex"
	"time"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/database/handle"
	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
)

// map[UserIdentity]define.UserData
var cachedSessionInfo = cache.New(time.Minute*30, time.Minute*5)

const (
	ValidateSessionStatusValidSession uint8 = iota
	ValidateSessionStatusInvalidSalt
	ValidateSessionStatusSaltNotSafe
	ValidateSessionStatusUserNotFound
	ValidateSessionStatusTokenInvalid
	ValidateSessionStatusTokenExpired
	ValidateSessionStatusUnknownError = 255
)

func LoadUser(userIdentity string, reload bool) (user *define.UserData, found bool, generalErr *define.GeneralError) {
	db := environment.DB.Database()
	userHandle := environment.DB.UserHandle()

	if !reload {
		if val, ok := cachedSessionInfo.Get(userIdentity); ok {
			return val.(*define.UserData), true, nil
		}
	}

	result, found, generalErr := userHandle.QueryUser(db, handle.QueryUserActionSearchByUserIdentity, userIdentity)
	if generalErr != nil {
		return nil, false, generalErr.AppendSource("LoadUser")
	}
	if !found {
		return nil, false, nil
	}

	cachedSessionInfo.Set(userIdentity, &result, cache.DefaultExpiration)
	return &result, true, nil
}

func UpdateLoginToken(userIdentity string, newToken string) *define.GeneralError {
	db := environment.DB.Database()
	userHandle := environment.DB.UserHandle()

	if generalErr := userHandle.UpdateLoginToken(db, userIdentity, newToken); generalErr != nil {
		return generalErr.AppendSource("UpdateLoginToken")
	}
	if _, _, generalErr := LoadUser(userIdentity, true); generalErr != nil {
		return generalErr.AppendSource("UpdateLoginToken")
	}

	return nil
}

func ExtendSession(userIdentity string) *define.GeneralError {
	db := environment.DB.Database()
	userHandle := environment.DB.UserHandle()

	if generalErr := userHandle.ExtendSession(db, userIdentity); generalErr != nil {
		return generalErr.AppendSource("ExtendSession")
	}
	if _, _, generalErr := LoadUser(userIdentity, true); generalErr != nil {
		return generalErr.AppendSource("ExtendSession")
	}

	return nil
}

func ValidateSession(session general.BasicSessionInfo) (status uint8, generalErr *define.GeneralError) {
	parsedSalt, err := uuid.Parse(session.RandomSalt)
	if err != nil {
		return ValidateSessionStatusInvalidSalt, nil
	}
	if parsedSalt == uuid.Nil {
		return ValidateSessionStatusSaltNotSafe, nil
	}

	user, found, generalErr := LoadUser(session.UserIdentity, false)
	if generalErr != nil {
		return ValidateSessionStatusUnknownError, generalErr.AppendSource("ValidateSession")
	}
	if !found {
		return ValidateSessionStatusUserNotFound, nil
	}

	checksum := sha512.Sum512([]byte(user.SessionInfo.LoginToken))
	checksum = sha512.Sum512(
		append(
			[]byte(hex.EncodeToString(checksum[:])),
			[]byte(parsedSalt.String())...,
		),
	)
	if hex.EncodeToString(checksum[:]) != session.EncryptedToken {
		return ValidateSessionStatusTokenInvalid, nil
	}
	if time.Now().Unix() > user.SessionInfo.ExpireUnixTime {
		return ValidateSessionStatusTokenExpired, nil
	}

	return ValidateSessionStatusValidSession, nil
}
