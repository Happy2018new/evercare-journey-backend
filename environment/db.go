package environment

import (
	"github.com/Happy2018new/evercare-journey-backend/database/handle"
	"go.etcd.io/bbolt"
	"gorm.io/gorm"
)

var DB *WrappedDatabase

const (
	DatabaseUser     = "username"
	DatabasePassword = "password"
	DatabaseHost     = "127.0.0.1:3306"
	DatabaseName     = "dbname"
)

type WrappedDatabase struct {
	database       *gorm.DB
	userHandle     *handle.UserHandle
	resourceHandle *handle.ResourceHandle
}

func NewWrappedDatabase(mysql *gorm.DB, bbolt *bbolt.DB) *WrappedDatabase {
	return &WrappedDatabase{
		database:       mysql,
		userHandle:     handle.NewUserHandle(),
		resourceHandle: handle.NewResourceHandle(bbolt),
	}
}

func (w *WrappedDatabase) Database() *gorm.DB {
	return w.database
}

func (w *WrappedDatabase) UserHandle() *handle.UserHandle {
	return w.userHandle
}

func (w *WrappedDatabase) ResourceHandle() *handle.ResourceHandle {
	return w.resourceHandle
}
