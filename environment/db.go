package environment

import (
	"github.com/Happy2018new/evercare-journey-backend/database/handle"
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
	database   *gorm.DB
	userHandle *handle.UserHandle
}

func NewWrappedDatabase(db *gorm.DB) *WrappedDatabase {
	return &WrappedDatabase{
		database:   db,
		userHandle: handle.NewUserHandle(),
	}
}

func (w *WrappedDatabase) Database() *gorm.DB {
	return w.database
}

func (w *WrappedDatabase) UserHandle() *handle.UserHandle {
	return w.userHandle
}
