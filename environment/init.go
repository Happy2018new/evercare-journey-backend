package environment

import (
	"fmt"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"go.etcd.io/bbolt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Initialize opens the configured databases and runs the schema migration.
// Callers should configure the exported settings before invoking this function.
func Initialize() {
	if DB != nil {
		return
	}
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		MySqlDatabaseUser,
		MySqlDatabasePassword,
		MySqlDatabaseAddress,
		MySqlDatabaseName,
	)
	mysqlDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		panic(fmt.Errorf("Failed to connect to MySQL: %w", err))
	}
	if err = define.AutoMigrateTable(mysqlDB); err != nil {
		panic(fmt.Errorf("Failed to migrate MySQL tables: %w", err))
	}

	bboltDatabasePath := BBoltDatabasePath()
	resDB, err := bbolt.Open(
		bboltDatabasePath,
		0600,
		&bbolt.Options{
			FreelistType: bbolt.FreelistMapType,
		},
	)
	if err != nil {
		panic(fmt.Errorf("Failed to open resource database %q: %w", bboltDatabasePath, err))
	}

	DB = NewWrappedDatabase(mysqlDB, resDB)
}
