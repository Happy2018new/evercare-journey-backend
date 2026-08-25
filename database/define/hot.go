package define

type HotPlace struct {
	HotPlaceUniqueID uint32 `gorm:"type:int unsigned;primaryKey"`
	HotPlaceIdentity string `gorm:"type:char(36);uniqueIndex"`

	RecommendTitle   string `gorm:"type:varchar(64);index"`
	RecommandDetail  string `gorm:"type:varchar(2048)"`
	PlaceImageItemID string `gorm:"type:char(36)"`

	PlaceIdentity string `gorm:"type:char(36);index"`
}
