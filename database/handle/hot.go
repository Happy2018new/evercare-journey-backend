package handle

import (
	"errors"
	"fmt"
	"strings"

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
	err := tx.Transaction(func(tx *gorm.DB) error {
		if generalErr := h.validateHotPlaceReferences(tx, hotPlace.PlaceIdentity, hotPlace.PlaceImageItemID, "CreateHotPlace"); generalErr != nil {
			return generalErr
		}
		result := tx.Create(&hotPlace)
		if result.Error != nil {
			return result.Error
		}
		return nil
	})
	if err == nil {
		return nil
	}
	if generalErr, ok := err.(*define.GeneralError); ok {
		return generalErr.AppendSource("CreateHotPlace")
	}
	return define.NewGeneralError("CreateHotPlace", err, define.LangKeyHotPlaceCreateUnknownErr)
}

func (h *HotHandle) validateHotPlaceReferences(tx *gorm.DB, placeIdentity string, imageItemID string, source string) *define.GeneralError {
	var place define.PlaceInfo
	result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("place_identity = ? AND place_status = ?", placeIdentity, define.PlaceStatusActive).
		First(&place)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return define.NewGeneralError(source, fmt.Errorf("referenced place is not active"), define.LangKeyHotPlacePlaceInvalid)
	}
	if result.Error != nil {
		return define.NewGeneralError(source, result.Error, define.LangKeyHotPlaceQueryUnknownErr)
	}
	if imageItemID == "" {
		return nil
	}
	if _, imageFound, err := h.resHandle.LoadResourceWithError(ResourceTypePlaceImage, imageItemID); err != nil {
		return define.NewGeneralError(source, err, define.LangKeyHotPlaceImageQueryUnknownErr)
	} else if !imageFound {
		return define.NewGeneralError(source, fmt.Errorf("referenced place image does not exist"), define.LangKeyHotPlaceImageQueryUnknownErr)
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
	default:
		return hotPlace, false, define.NewGeneralError("QueryHotPlace", fmt.Errorf("unsupported action %d", action), define.LangKeyHotPlaceQueryUnknownErr)
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

// QueryMulActivePlace returns only recommendations whose referenced place is
// still present and active. A hot recommendation is metadata, so it must not
// make a provider request just to decide whether it is safe to show.
func (h *HotHandle) QueryMulActivePlace(tx *gorm.DB, title string) (mulPlace []define.HotPlace, generalErr *define.GeneralError) {
	query := tx
	if strings.TrimSpace(title) != "" {
		query = query.Where("recommend_title LIKE ?", "%"+strings.TrimSpace(title)+"%")
	}
	activePlaceIDs := tx.Model(&define.PlaceInfo{}).
		Select("place_identity").
		Where("place_status = ?", define.PlaceStatusActive)
	result := query.
		Where("place_identity IN (?)", activePlaceIDs).
		Order("recommend_title ASC, place_identity ASC").
		Find(&mulPlace)
	if result.Error != nil {
		return nil, define.NewGeneralError("QueryMulActivePlace", result.Error, define.LangKeyHotPlaceQueryUnknownErr)
	}
	return mulPlace, nil
}

func (h *HotHandle) IsPlaceActive(tx *gorm.DB, placeIdentity string) (bool, *define.GeneralError) {
	var place define.PlaceInfo
	result := tx.
		Where("place_identity = ? AND place_status = ?", placeIdentity, define.PlaceStatusActive).
		First(&place)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if result.Error != nil {
		return false, define.NewGeneralError("IsPlaceActive", result.Error, define.LangKeyHotPlaceQueryUnknownErr)
	}
	return true, nil
}

func (h *HotHandle) UpdateHotPlace(
	tx *gorm.DB,
	hotPlaceIdentity string,
	recommendTitle string,
	recommandDetail string,
	placeImageItemID string,
	placeIdentity string,
) *define.GeneralError {
	var oldImageItemID string
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
		if generalErr := h.validateHotPlaceReferences(tx, placeIdentity, placeImageItemID, "UpdateHotPlace"); generalErr != nil {
			return generalErr
		}
		oldImageItemID = hotPlace.PlaceImageItemID

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
		if oldImageItemID != "" && oldImageItemID != placeImageItemID {
			var references int64
			if result := tx.Model(&define.HotPlace{}).Where("place_image_item_id = ?", oldImageItemID).Count(&references); result.Error == nil && references == 0 {
				_ = h.resHandle.DeleteResource(ResourceTypePlaceImage, oldImageItemID)
			}
		}
		return nil
	}
	if generalErr, ok := err.(*define.GeneralError); ok {
		return generalErr.AppendSource("UpdateHotPlace")
	}

	return define.NewGeneralError("UpdateHotPlace", err, define.LangKeyHotPlaceUpdateUnknownErr)
}

func (h *HotHandle) DeleteHotPlace(tx *gorm.DB, hotPlaceIdentity string) *define.GeneralError {
	var imageItemID string
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

		imageItemID = hotPlace.PlaceImageItemID
		return nil
	})

	if err == nil {
		if imageItemID != "" {
			var references int64
			if result := tx.Model(&define.HotPlace{}).Where("place_image_item_id = ?", imageItemID).Count(&references); result.Error == nil && references == 0 {
				if deleteErr := h.resHandle.DeleteResource(ResourceTypePlaceImage, imageItemID); deleteErr != nil {
					return define.NewGeneralError("DeleteHotPlace", deleteErr, define.LangKeyHotPlaceImageDeleteUnknownErr)
				}
			}
		}
		return nil
	}
	if generalErr, ok := err.(*define.GeneralError); ok {
		return generalErr.AppendSource("DeleteHotPlace")
	}

	return define.NewGeneralError("DeleteHotPlace", err, define.LangKeyHotPlaceDeleteUnknownErr)
}
