package define

import (
	"fmt"

	"gorm.io/gorm"
)

func AutoMigrateTable(database *gorm.DB) error {
	err := database.AutoMigrate(
		&UserData{},
		&UserProfile{},
		&UserSession{},
		&PlaceInfo{},
		&TripInfo{},
		&FamilyInfo{},
		&FamilyMember{},
		&FamilyPinnedTrip{},
		&HotPlace{},
		&MessageInfo{},
		&MessageRecipient{},
		&SafetySetting{},
	)
	if err != nil {
		return fmt.Errorf("AutoMigrateTable: %w", err)
	}
	return nil
}
