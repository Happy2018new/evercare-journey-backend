package handle

import (
	"fmt"

	"go.etcd.io/bbolt"
)

const (
	ResourceTypeUserAvatar = "USER_AVATAR"
)

type ResourceHandle struct {
	db *bbolt.DB
}

func NewResourceHandle(db *bbolt.DB) *ResourceHandle {
	return &ResourceHandle{
		db: db,
	}
}

func (r *ResourceHandle) LoadResource(typeName string, resKey string) (result []byte, found bool) {
	_ = r.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(typeName))
		if bucket == nil {
			return nil
		}
		if result = bucket.Get([]byte(resKey)); result != nil {
			result = append([]byte(nil), result...)
			found = true
		}
		return nil
	})
	return result, found
}

func (r *ResourceHandle) SaveResource(typeName string, resKey string, data []byte) error {
	err := r.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(typeName))
		if err != nil {
			return err
		}
		return bucket.Put([]byte(resKey), data)
	})
	if err != nil {
		return fmt.Errorf("SaveResource: %w", err)
	}
	return nil
}

func (r *ResourceHandle) DeleteResource(typeName string, resKey string) error {
	err := r.db.Update(func(tx *bbolt.Tx) error {
		if bucket := tx.Bucket([]byte(typeName)); bucket != nil {
			return bucket.Delete([]byte(resKey))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("DeleteResource: %w", err)
	}
	return nil
}
