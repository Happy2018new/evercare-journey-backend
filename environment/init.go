package environment

import (
	"fmt"

	"go.etcd.io/bbolt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func init() {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		MySqlDatabaseUser,
		MySqlDatabasePassword,
		MySqlDatabaseAddress,
		MySqlDatabaseName,
	)
	mysql, err := gorm.Open(mysql.Open(dsn))
	if err != nil {
		panic(fmt.Errorf("Failed to connect to MySQL: %w", err))
	}

	bbolt, err := bbolt.Open(
		BBoltDatabasePath,
		0600,
		&bbolt.Options{
			FreelistType: bbolt.FreelistMapType,
		},
	)
	if err != nil {
		panic(fmt.Errorf("Failed to open resource database: %w", err))
	}

	DB = NewWrappedDatabase(mysql, bbolt)
}
