package handle

import (
	"fmt"

	"go.etcd.io/bbolt"
)

const (
	ResourceTypeNameUserAvatar = "USER_AVATAR"
)

type ResourceHandle struct {
	db *bbolt.DB
}

func NewResourceHandle(db *bbolt.DB) *ResourceHandle {
	return &ResourceHandle{
		db: db,
	}
}

func (r *ResourceHandle) LoadResource(typeName string, srcKey string) (result []byte, found bool) {
	_ = r.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(typeName))
		if bucket == nil {
			return nil
		}
		if result = bucket.Get([]byte(srcKey)); result != nil {
			result = append([]byte(nil), result...)
			found = true
		}
		return nil
	})
	return result, found
}

func (r *ResourceHandle) SaveResource(typeName string, srcKey string, data []byte) error {
	err := r.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(typeName))
		if err != nil {
			return err
		}
		if err = bucket.Put([]byte(srcKey), data); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("SaveResource: %w", err)
	}
	return nil
}
