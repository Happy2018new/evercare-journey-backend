package define

const (
	FamilyMemberPermissionNormal uint8 = iota
	FamilyMemberPermissionAdmin
)

type FamilyInfo struct {
	FamilyUniqueID    uint64 `gorm:"primaryKey;type:bigint unsigned;autoIncrement"`
	FamilyIdentity    string `gorm:"type:char(36);uniqueIndex"`
	FamilyName        string `gorm:"type:varchar(64)"`
	OwnerUserUniqueID uint32 `gorm:"type:int unsigned;index"`
	CreatedUnixTime   int64  `gorm:"type:bigint"`
	UpdateUnixTime    int64  `gorm:"type:bigint"`
}

type FamilyMember struct {
	FamilyMemberUniqueID uint64 `gorm:"primaryKey;type:bigint unsigned;autoIncrement"`
	FamilyUniqueID       uint64 `gorm:"type:bigint unsigned;uniqueIndex:idx_family_member,priority:1"`
	UserUniqueID         uint32 `gorm:"type:int unsigned;uniqueIndex:idx_family_member,priority:2;index"`
	PermissionLevel      uint8  `gorm:"type:tinyint unsigned"`
	JoinedUnixTime       int64  `gorm:"type:bigint"`
}

type FamilyPinnedTrip struct {
	FamilyPinnedTripUniqueID uint64 `gorm:"primaryKey;type:bigint unsigned;autoIncrement"`
	FamilyUniqueID           uint64 `gorm:"type:bigint unsigned;uniqueIndex:idx_family_pinned_trip,priority:1"`
	TripIdentity             string `gorm:"type:char(36);uniqueIndex:idx_family_pinned_trip,priority:2;index"`
	TripOwnerUserUniqueID    uint32 `gorm:"type:int unsigned;index"`
	PinnedByUserUniqueID     uint32 `gorm:"type:int unsigned;index"`
	CreatedUnixTime          int64  `gorm:"type:bigint"`
}
