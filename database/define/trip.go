package define

import "time"

const (
	PlaceProviderDefault         = "amap"
	PlaceCoordinateSystemDefault = "gcj02"
)

const (
	PlaceStatusActive uint8 = iota
	PlaceStatusUnavailable
	PlaceStatusArchived
)

const (
	TripNodeTypeStart uint8 = iota
	TripNodeTypeWaypoint
	TripNodeTypeEnd
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

type Place struct {
	PlaceUniqueID   uint64 `gorm:"primaryKey;type:bigint unsigned;autoIncrement"`
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

	PlaceStatus      uint8 `gorm:"type:tinyint unsigned"`
	LastSyncUnixTime int64 `gorm:"type:bigint"`
}

type TripNode struct {
	TripNodeUniqueID uint64 `gorm:"primaryKey;type:bigint unsigned;autoIncrement"`

	BelongToWhichTrip uint64 `gorm:"type:bigint unsigned;index:idx_trip_node_ind,priority:1"`
	LinkToWhichPlace  uint64 `gorm:"type:bigint unsigned;index"`

	NodeIndex uint8  `gorm:"type:tinyint unsigned;index:idx_trip_node_ind,priority:2"`
	NodeType  uint8  `gorm:"type:tinyint unsigned"`
	NodeNote  string `gorm:"type:varchar(500)"`

	PlaceData Place `gorm:"foreignKey:LinkToWhichPlace;references:PlaceUniqueID"`
}

type Trip struct {
	TripUniqueID uint64 `gorm:"primaryKey;type:bigint unsigned;autoIncrement"`
	UserUniqueID uint32 `gorm:"type:int unsigned;index"`

	TripName   string    `gorm:"type:varchar(14)"`
	TripDate   time.Time `gorm:"type:date;index"`
	TravelMode uint8     `gorm:"type:tinyint unsigned"`
	TripStatus uint8     `gorm:"type:tinyint unsigned"`

	CreateUnixTime int64 `gorm:"type:bigint"`
	UpdateUnixTime int64 `gorm:"type:bigint"`

	OwnerData UserData   `gorm:"foreignKey:UserUniqueID;references:UserUniqueID"`
	NodeData  []TripNode `gorm:"foreignKey:BelongToWhichTrip;references:TripUniqueID"`
}
