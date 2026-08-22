package define

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	UserPermissionDefault        = UserPermissionNormal
	ExtendSessionDurationDefault = 30 * 24 * 60 * 60
)

const (
	UserPermissionNormal uint8 = iota
	UserPermissionAdvance
	UserPermissionAdmin
	UserPermissionSystem
)

const (
	UserGenderNotSet uint8 = iota
	UserGenderMan
	UserGenderWoman
)

const (
	UserAgeNotSet uint8 = 255
	UserMinAge    uint8 = 0
	UserMaxAge    uint8 = 150
)

type UserData struct {
	UserUniqueID    uint32      `gorm:"primaryKey;type:int;autoIncrement"`
	UserIdentity    string      `gorm:"type:char(36);uniqueIndex"`
	AccountName     string      `gorm:"type:varchar(14);uniqueIndex"`
	AccountPhone    string      `gorm:"type:varchar(20);uniqueIndex"`
	PermissionLevel uint8       `gorm:"type:tinyint unsigned"`
	SessionInfo     UserSession `gorm:"foreignKey:UserUniqueID;references:UserUniqueID"`
	ProfileData     UserProfile `gorm:"foreignKey:UserUniqueID;references:UserUniqueID"`
}

type UserProfile struct {
	UserUniqueID uint32 `gorm:"primaryKey;type:int"`
	AvatarItemID string `gorm:"type:varchar(36)"`
	Gender       uint8  `gorm:"type:tinyint unsigned"`
	Age          uint8  `gorm:"type:tinyint unsigned"`
}

type UserSession struct {
	UserUniqueID   uint32 `gorm:"primaryKey;type:int"`
	LoginToken     string `gorm:"type:char(36);uniqueIndex"`
	ExpireUnixTime int64  `gorm:"type:bigint"`
}

func MakeNewUser(accountPhone string, permissionLevel uint8) UserData {
	return UserData{
		UserIdentity:    uuid.NewString(),
		AccountName:     strings.ReplaceAll(uuid.NewString(), "-", "")[:14],
		AccountPhone:    accountPhone,
		PermissionLevel: permissionLevel,
		SessionInfo: UserSession{
			LoginToken:     uuid.NewString(),
			ExpireUnixTime: time.Now().Unix() + ExtendSessionDurationDefault,
		},
		ProfileData: UserProfile{
			Gender: UserGenderNotSet,
			Age:    UserAgeNotSet,
		},
	}
}

func IsValidAccountName(name string) (valid bool, reason string) {
	length := utf8.RuneCountInString(name)
	if length < 3 || length > 14 {
		return false, LangKeyGeneralNameInvalidLen
	}
	for _, char := range name {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '_' {
			continue
		}
		return false, LangKeyGeneralNameInvalidChar
	}
	return true, ""
}
