package define

const UserPermissionDefault = UserPermissionNormal

const (
	UserPermissionNormal uint8 = iota
	UserPermissionAdvance
	UserPermissionAdmin
	UserPermissionSystem
)

type UserData struct {
	UserUniqueID    uint32      `gorm:"primaryKey;type:int;autoIncrement"`
	UserIdentity    string      `gorm:"type:char(36);uniqueIndex"`
	AccountName     string      `gorm:"type:varchar(14);uniqueIndex"`
	AccountPassword []byte      `gorm:"type:binary(64)"`
	AccountPhone    string      `gorm:"type:varchar(20);uniqueIndex"`
	PermissionLevel uint8       `gorm:"type:int"`
	SessionInfo     UserSession `gorm:"foreignKey:UserUniqueID;references:UserUniqueID"`
	ProfileData     UserProfile `gorm:"foreignKey:UserUniqueID;references:UserUniqueID"`
}

type UserProfile struct {
	UserUniqueID uint32 `gorm:"primaryKey;type:int"`
	AvatarItemID string `gorm:"type:char(36)"`
	Gender       uint8  `gorm:"type:int"`
	Age          uint8  `gorm:"type:int"`
}

type UserSession struct {
	UserUniqueID   uint32 `gorm:"primaryKey;type:int"`
	LoginToken     string `gorm:"type:char(36);uniqueIndex"`
	ExpireUnixTime int64  `gorm:"type:bigint"`
}
