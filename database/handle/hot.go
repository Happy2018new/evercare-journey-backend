package handle

import (
	"context"
	"errors"
	"fmt"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	QueryHotPlaceActionSearchByUniqueID uint8 = iota
	QueryHotPlaceActionSearchByIdentity
	QueryHotPlaceActionSearchByTitle
)

type HotHandle struct {
	resHandle  *ResourceHandle
	tripHandle *TripHandle
}

func NewHotHandle(r *ResourceHandle, t *TripHandle) *HotHandle {
	return &HotHandle{
		resHandle:  r,
		tripHandle: t,
	}
}

func (h *HotHandle) CreateHotPlace(
	tx *gorm.DB,
	hotPlace define.HotPlace,
) *define.GeneralError {
	_, found, generalErr := h.tripHandle.QueryPlace(
		context.Background(),
		tx,
		QueryPlaceActionSearchByIdentity,
		hotPlace.PlaceIdentity,
	)
	if generalErr != nil {
		return generalErr.AppendSource("CreateHotPlace")
	}
	if !found {
		return define.NewGeneralError("CreateHotPlace", fmt.Errorf("Referenced place does not exist"), define.LangKeyHotPlaceCreateUnknownErr)
	}

	result := tx.Create(&hotPlace)
	if result.Error != nil {
		return define.NewGeneralError("CreateHotPlace", result.Error, define.LangKeyGeneralUnknownErr)
	}

	return nil
}

func (h *HotHandle) QueryHotPlace(
	tx *gorm.DB,
	action uint8,
	keyword any,
) (hotPlace define.HotPlace, found bool, generalErr *define.GeneralError) {
	query := tx

	switch action {
	case QueryHotPlaceActionSearchByUniqueID:
		query = query.Where("hot_place_unique_id = ?", keyword)
	case QueryHotPlaceActionSearchByIdentity:
		query = query.Where("hot_place_identity = ?", keyword)
	case QueryHotPlaceActionSearchByTitle:
		query = query.Where("recommend_title = ?", keyword)
	}

	result := query.First(&hotPlace)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return hotPlace, false, nil
	}
	if result.Error != nil {
		return hotPlace, false, define.NewGeneralError("QueryHotPlace", result.Error, define.LangKeyHotPlaceQueryUnknownErr)
	}

	return hotPlace, true, nil
}

func (h *HotHandle) QueryMulPlace(tx *gorm.DB, title string) (mulPlace []define.HotPlace, generalErr *define.GeneralError) {
	query := tx
	if len(title) > 0 {
		query = query.Where("recommend_title LIKE ?", "%"+title+"%")
	}

	result := query.
		Order("recommend_title ASC, place_identity ASC").
		Find(&mulPlace)
	if result.Error != nil {
		return nil, define.NewGeneralError("QueryMulPlace", result.Error, define.LangKeyHotPlaceQueryUnknownErr)
	}

	return mulPlace, nil
}

func (h *HotHandle) UpdateHotPlace(
	tx *gorm.DB,
	hotPlaceIdentity string,
	recommendTitle string,
	recommandDetail string,
	placeImageItemID string,
	placeIdentity string,
) *define.GeneralError {
	err := tx.Transaction(func(tx *gorm.DB) error {
		hotPlace, found, generalErr := h.QueryHotPlace(
			tx.Clauses(clause.Locking{Strength: "UPDATE"}),
			QueryHotPlaceActionSearchByIdentity,
			hotPlaceIdentity,
		)
		if generalErr != nil {
			return generalErr
		}
		if !found {
			return define.NewGeneralError("", fmt.Errorf("Target hot place not found"), define.LangKeyHotPlaceUpdateNotFoundErr)
		}

		result := tx.Model(&hotPlace).Updates(map[string]any{
			"recommend_title":     recommendTitle,
			"recommand_detail":    recommandDetail,
			"place_image_item_id": placeImageItemID,
			"place_identity":      placeIdentity,
		})
		if result.Error != nil {
			return result.Error
		}

		return nil
	})

	if err == nil {
		return nil
	}
	if generalErr, ok := err.(*define.GeneralError); ok {
		return generalErr.AppendSource("UpdateHotPlace")
	}

	return define.NewGeneralError("UpdateHotPlace", err, define.LangKeyHotPlaceUpdateUnknownErr)
}

func (h *HotHandle) DeleteHotPlace(tx *gorm.DB, hotPlaceIdentity string) *define.GeneralError {
	err := tx.Transaction(func(tx *gorm.DB) error {
		var hotPlace define.HotPlace

		result := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("hot_place_identity = ?", hotPlaceIdentity).
			First(&hotPlace)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return define.NewGeneralError("DeleteHotPlace", fmt.Errorf("Target hot place not found"), define.LangKeyHotPlaceDeleteNotFoundErr)
		}
		if result.Error != nil {
			return result.Error
		}

		result = tx.Delete(&hotPlace)
		if result.Error != nil {
			return result.Error
		}

		return h.resHandle.DeleteResource(
			ResourceTypePlaceImage,
			hotPlace.PlaceImageItemID,
		)
	})

	if err == nil {
		return nil
	}
	if generalErr, ok := err.(*define.GeneralError); ok {
		return generalErr.AppendSource("DeleteHotPlace")
	}

	return define.NewGeneralError("DeleteHotPlace", err, define.LangKeyHotPlaceDeleteUnknownErr)
}
