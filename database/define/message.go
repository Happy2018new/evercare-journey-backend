package define

import "time"

const (
	MessageTypeAnnouncement uint8 = iota
	MessageTypeSOS
	MessageTypeChat
)

// MessageInfo stores a message once per family. Delivery and read state are
// kept separately because every family member has an independent read state.
type MessageInfo struct {
	MessageUniqueID     uint64    `gorm:"primaryKey;type:bigint unsigned;autoIncrement"`
	MessageIdentity     string    `gorm:"type:char(36);uniqueIndex"`
	FamilyUniqueID      uint64    `gorm:"type:bigint unsigned;index:idx_message_family_created,priority:1"`
	SenderUserUniqueID  uint32    `gorm:"type:int unsigned;index"`
	MessageType         uint8     `gorm:"type:tinyint unsigned;index"`
	Title               string    `gorm:"type:varchar(128)"`
	Content             string    `gorm:"type:text"`
	RelatedTripIdentity string    `gorm:"type:char(36);index"`
	CreatedUnixTime     int64     `gorm:"type:bigint;index:idx_message_family_created,priority:2"`
	CreatedAt           time.Time `gorm:"-" json:"-"`
}

type MessageRecipient struct {
	MessageRecipientUniqueID uint64 `gorm:"primaryKey;type:bigint unsigned;autoIncrement"`
	MessageUniqueID          string `gorm:"type:char(36);uniqueIndex:idx_message_recipient,priority:1;index"`
	RecipientUserUniqueID    uint32 `gorm:"type:int unsigned;uniqueIndex:idx_message_recipient,priority:2;index"`
	ReadUnixTime             int64  `gorm:"type:bigint"`
}
