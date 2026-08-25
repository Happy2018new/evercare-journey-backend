package environment

import (
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
	BBoltDatabasePath     = "res.db"
)

type WrappedDatabase struct {
	database       *gorm.DB
	userHandle     *handle.UserHandle
	tripHandle     *handle.TripHandle
	resourceHandle *handle.ResourceHandle
}

func NewWrappedDatabase(mysql *gorm.DB, bbolt *bbolt.DB) *WrappedDatabase {
	resHandle := handle.NewResourceHandle(bbolt)
	wrappedDB := &WrappedDatabase{
		database:       mysql,
		userHandle:     handle.NewUserHandle(),
		tripHandle:     handle.NewTripHandle(resHandle),
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

func (w *WrappedDatabase) ResourceHandle() *handle.ResourceHandle {
	return w.resourceHandle
}
