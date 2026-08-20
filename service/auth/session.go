package auth

import (
	"crypto/sha512"
	"encoding/hex"
	"time"

	"github.com/Happy2018new/evercare-journey-backed/database/define"
	"github.com/Happy2018new/evercare-journey-backed/database/handle"
	"github.com/Happy2018new/evercare-journey-backed/environment"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
)

// map[UserIdentity]tokenWithExpireTime
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

type TokenWithExpireTime struct {
	LoginToken     string
	ExpireUnixTime int64
}

func LoadLoginToken(userIdentity string, reload bool) (loginToken TokenWithExpireTime, found bool, generalErr *define.GeneralError) {
	db := environment.DB.Database()
	userHandle := environment.DB.UserHandle()

	if !reload {
		if val, ok := cachedSessionInfo.Get(userIdentity); ok {
			return val.(TokenWithExpireTime), true, nil
		}
	}

	user, found, generalErr := userHandle.QueryUser(db, handle.QueryUserActionSearchByUserIdentity, userIdentity)
	if generalErr != nil {
		return loginToken, false, generalErr.AppendSource("LoadLoginToken")
	}
	if !found {
		return loginToken, false, nil
	}

	loginToken = TokenWithExpireTime{
		LoginToken:     user.SessionInfo.LoginToken,
		ExpireUnixTime: user.SessionInfo.ExpireUnixTime,
	}
	cachedSessionInfo.Set(userIdentity, loginToken, cache.DefaultExpiration)
	return loginToken, true, nil
}

func UpdateLoginToken(userIdentity string, newToken string) *define.GeneralError {
	db := environment.DB.Database()
	userHandle := environment.DB.UserHandle()

	if generalErr := userHandle.UpdateLoginToken(db, userIdentity, newToken); generalErr != nil {
		return generalErr.AppendSource("UpdateLoginToken")
	}
	if _, _, generalErr := LoadLoginToken(userIdentity, true); generalErr != nil {
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
	if _, _, generalErr := LoadLoginToken(userIdentity, true); generalErr != nil {
		return generalErr.AppendSource("ExtendSession")
	}

	return nil
}

func ValidateSession(userIdentity string, salt string, token string) (status uint8, generalErr *define.GeneralError) {
	parsedSalt, err := uuid.Parse(salt)
	if err != nil {
		return ValidateSessionStatusInvalidSalt, nil
	}
	if parsedSalt == uuid.Nil {
		return ValidateSessionStatusSaltNotSafe, nil
	}

	loginToken, found, generalErr := LoadLoginToken(userIdentity, false)
	if generalErr != nil {
		return ValidateSessionStatusUnknownError, generalErr.AppendSource("ValidateSession")
	}
	if !found {
		return ValidateSessionStatusUserNotFound, nil
	}

	checksum := sha512.Sum512([]byte(loginToken.LoginToken))
	checksum = sha512.Sum512(
		append(
			[]byte(hex.EncodeToString(checksum[:])),
			[]byte(salt)...,
		),
	)
	if hex.EncodeToString(checksum[:]) != token {
		return ValidateSessionStatusTokenInvalid, nil
	}
	if time.Now().Unix() > loginToken.ExpireUnixTime {
		return ValidateSessionStatusTokenExpired, nil
	}

	return ValidateSessionStatusValidSession, nil
}
