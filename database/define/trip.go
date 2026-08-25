package define

import (
	"time"

	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

const (
	PlaceProviderNameDefault     = "amap"
	PlaceCoordinateSystemDefault = "gcj02"
)

const (
	TripNameMaxLengthDefault  = 14
	TripCurrentVersionDefault = 1
)

const (
	PlaceStatusActive uint8 = iota
	PlaceStatusUnavailable
	PlaceStatusArchived
)

const (
	TripStatusInPlanning uint8 = iota
	TripStatusInProgress
	TripStatusCompleted
	TripStatusCancelled
)

const (
	TripTravelModeWalking uint8 = iota
	TripTravelModeDriving
	TripTravelModeTransit
)

type PlaceInfo struct {
	PlaceUniqueID   uint64 `gorm:"primaryKey;type:bigint unsigned;autoIncrement"`
	PlaceIdentity   string `gorm:"type:char(36);uniqueIndex"`
	ProviderName    string `gorm:"type:varchar(16);uniqueIndex:idx_place_provider_id,priority:1"`
	ProviderPlaceID string `gorm:"type:varchar(64);uniqueIndex:idx_place_provider_id,priority:2"`

	PlaceName    string `gorm:"type:varchar(128);index"`
	CategoryCode string `gorm:"type:varchar(32);index"`
	CategoryName string `gorm:"type:varchar(128)"`

	FullAddress     string `gorm:"type:varchar(255)"`
	InWhichProvince string `gorm:"type:varchar(64)"`
	InWhichCity     string `gorm:"type:varchar(64)"`
	InWhichDistrict string `gorm:"type:varchar(64)"`
	AdCode          string `gorm:"type:varchar(16);index"`

	Longitude        float64 `gorm:"type:decimal(10,7)"`
	Latitude         float64 `gorm:"type:decimal(10,7)"`
	CoordinateSystem string  `gorm:"type:varchar(16)"`

	PlaceStatus  uint8 `gorm:"type:tinyint unsigned"`
	SyncUnixTime int64 `gorm:"type:bigint"`
}

type TripInfo struct {
	TripUniqueID uint64 `gorm:"primaryKey;type:bigint unsigned;autoIncrement"`
	TripIdentity string `gorm:"type:char(36);uniqueIndex"`
	UserUniqueID uint32 `gorm:"type:int unsigned;index"`

	TripName   string    `gorm:"type:varchar(14)"`
	TripDate   time.Time `gorm:"type:date;index"`
	TravelMode uint8     `gorm:"type:tinyint unsigned"`
	TripStatus uint8     `gorm:"type:tinyint unsigned"`

	CurrentVersion uint32 `gorm:"type:int unsigned"`
	UpdateUnixTime int64  `gorm:"type:bigint"`

	OwnerInfo UserData `gorm:"foreignKey:UserUniqueID;references:UserUniqueID"`
}

type TripNode struct {
	NoteString    uuid.UUID `json:"note_string"`
	PlaceIdentity string    `json:"place_identity"`
}

func NewTripNode() *TripNode {
	return new(TripNode)
}

func (t *TripNode) Marshal(io protocol.IO) {
	io.UUID(&t.NoteString)
	io.StringUTF(&t.PlaceIdentity)
}

type MulTripNode []TripNode

func (m *MulTripNode) Marshal(io protocol.IO) {
	data := ([]TripNode)(*m)
	protocol.SliceUint8Length(io, &data)
	*m = data
}
