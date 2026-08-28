package environment

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/Happy2018new/evercare-journey-backend/database/handle"
	"go.etcd.io/bbolt"
	"gorm.io/gorm"
)

var DB *WrappedDatabase

const (
	MySqlDatabaseUser     = "username"
	MySqlDatabasePassword = "password"
	MySqlDatabaseAddress  = "127.0.0.1:3306"
	MySqlDatabaseName     = "dbname"
	BBoltDatabaseFileName = "res.db"
)

// BBoltDatabasePath resolves the resource database independently of the
// process working directory. Set EVERCARE_BBOLT_DATABASE_PATH to override it.
func BBoltDatabasePath() string {
	if configuredPath := os.Getenv("EVERCARE_BBOLT_DATABASE_PATH"); configuredPath != "" {
		return configuredPath
	}
	if _, sourceFile, _, ok := runtime.Caller(0); ok {
		return filepath.Join(filepath.Dir(filepath.Dir(sourceFile)), BBoltDatabaseFileName)
	}
	return BBoltDatabaseFileName
}

type WrappedDatabase struct {
	database       *gorm.DB
	userHandle     *handle.UserHandle
	tripHandle     *handle.TripHandle
	familyHandle   *handle.FamilyHandle
	hotHandle      *handle.HotHandle
	resourceHandle *handle.ResourceHandle
}

func NewWrappedDatabase(mysql *gorm.DB, bbolt *bbolt.DB) *WrappedDatabase {
	resHandle := handle.NewResourceHandle(bbolt)
	tripHandle := handle.NewTripHandle(resHandle)

	wrappedDB := &WrappedDatabase{
		database:       mysql,
		userHandle:     handle.NewUserHandle(),
		tripHandle:     tripHandle,
		familyHandle:   handle.NewFamilyHandle(),
		hotHandle:      handle.NewHotHandle(resHandle, tripHandle),
		resourceHandle: resHandle,
	}

	return wrappedDB
}

func (w *WrappedDatabase) Database() *gorm.DB {
	return w.database
}

func (w *WrappedDatabase) UserHandle() *handle.UserHandle {
	return w.userHandle
}

func (w *WrappedDatabase) TripHandle() *handle.TripHandle {
	return w.tripHandle
}

func (w *WrappedDatabase) FamilyHandle() *handle.FamilyHandle { return w.familyHandle }

func (w *WrappedDatabase) HotHandle() *handle.HotHandle {
	return w.hotHandle
}

func (w *WrappedDatabase) ResourceHandle() *handle.ResourceHandle {
	return w.resourceHandle
}
