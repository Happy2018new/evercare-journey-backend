package define

type SafetySetting struct {
	UserUniqueID           uint32 `gorm:"primaryKey;type:int unsigned"`
	EmergencyLocationShare bool   `gorm:"not null;default:false"`
	SOSConfirmation        bool   `gorm:"not null;default:true"`
}
