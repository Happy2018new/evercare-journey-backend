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
	)
	if err != nil {
		return fmt.Errorf("AutoMigrateTable: %w", err)
	}
	return nil
}
