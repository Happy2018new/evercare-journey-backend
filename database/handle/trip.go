package handle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/utils"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PlaceRefreshDefaultTime = 1800
)

const (
	QueryPlaceActionSearchByUniqueID uint8 = iota
	QueryPlaceActionSearchByIdentity
	QueryPlaceActionSearchByAmapPlaceID
)

type TripHandle struct {
	resHandle *ResourceHandle
}

func NewTripHandle(r *ResourceHandle) *TripHandle {
	return &TripHandle{
		resHandle: r,
	}
}

func FromUtilsPlace(place utils.AmapPlace) define.PlaceInfo {
	return define.PlaceInfo{
		ProviderName:     define.PlaceProviderNameDefault,
		ProviderPlaceID:  place.ProviderPlaceID,
		PlaceName:        place.Name,
		CategoryCode:     place.CategoryCode,
		CategoryName:     place.CategoryName,
		FullAddress:      place.FullAddress,
		InWhichProvince:  place.ProvinceName,
		InWhichCity:      place.CityName,
		InWhichDistrict:  place.DistrictName,
		AdCode:           place.AdCode,
		Longitude:        place.Longitude,
		Latitude:         place.Latitude,
		CoordinateSystem: define.PlaceCoordinateSystemDefault,
		PlaceStatus:      define.PlaceStatusActive,
	}
}

func (t *TripHandle) StorePlace(tx *gorm.DB, place define.PlaceInfo) (final define.PlaceInfo, generalErr *define.GeneralError) {
	var existing define.PlaceInfo

	place.SyncUnixTime = time.Now().Unix()
	if place.PlaceIdentity == "" {
		place.PlaceIdentity = uuid.NewString()
	}

	result := tx.
		Where("place_identity = ?", place.PlaceIdentity).
		First(&existing)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		if err := tx.Create(&place).Error; err != nil {
			return place, define.NewGeneralError("StorePlace", err, define.LangKeyPlaceSaveUnknownErr)
		}
		return place, nil
	}
	if result.Error != nil {
		return place, define.NewGeneralError("StorePlace", result.Error, define.LangKeyPlaceQueryUnknownErr)
	}

	result = tx.
		Model(&existing).
		Updates(map[string]any{
			"place_name":        place.PlaceName,
			"category_code":     place.CategoryCode,
			"category_name":     place.CategoryName,
			"full_address":      place.FullAddress,
			"in_which_province": place.InWhichProvince,
			"in_which_city":     place.InWhichCity,
			"in_which_district": place.InWhichDistrict,
			"ad_code":           place.AdCode,
			"longitude":         place.Longitude,
			"latitude":          place.Latitude,
			"coordinate_system": place.CoordinateSystem,
			"place_status":      place.PlaceStatus,
			"sync_unix_time":    place.SyncUnixTime,
		})
	if result.Error != nil {
		return place, define.NewGeneralError("StorePlace", result.Error, define.LangKeyPlaceSaveUnknownErr)
	}

	place.PlaceUniqueID = existing.PlaceUniqueID
	return place, nil
}

func (t *TripHandle) QueryPlace(
	ctx context.Context,
	tx *gorm.DB,
	action uint8,
	keyword any,
) (place define.PlaceInfo, found bool, generalErr *define.GeneralError) {
	var query *gorm.DB

	switch action {
	case QueryPlaceActionSearchByUniqueID:
		query = tx.Where("place_unique_id = ?", keyword)
	case QueryPlaceActionSearchByIdentity:
		query = tx.Where("place_identity = ?", keyword)
	case QueryPlaceActionSearchByAmapPlaceID:
		query = tx.Where("provider_name = ? AND provider_place_id = ?", define.PlaceProviderNameDefault, keyword)
	default:
		return place, false, define.NewGeneralError(
			"QueryPlace",
			fmt.Errorf("Unsupported action %d", action),
			define.LangKeyPlaceQueryUnknownErr,
		)
	}

	result := query.First(&place)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return place, false, nil
	}
	if result.Error != nil {
		return place, false, define.NewGeneralError(
			"QueryPlace",
			fmt.Errorf("action = %d, keyword = %v, err = %w", action, keyword, result.Error),
			define.LangKeyPlaceQueryUnknownErr,
		)
	}
	if time.Now().Unix() <= place.SyncUnixTime+PlaceRefreshDefaultTime {
		return place, true, nil
	}

	amapPlace, err := utils.GetAmapPlaceByID(ctx, place.ProviderPlaceID)
	if err != nil {
		return define.PlaceInfo{}, true, define.NewGeneralError(
			"QueryPlace",
			fmt.Errorf("Failed to refresh place %s due to %w", place.PlaceIdentity, err),
			define.LangKeyPlaceRefreshUnknownErr,
		)
	}
	placeInfo := FromUtilsPlace(amapPlace)
	placeInfo.PlaceIdentity = place.PlaceIdentity
	placeInfo.PlaceUniqueID = place.PlaceUniqueID
	placeInfo, generalErr = t.StorePlace(tx, placeInfo)
	if generalErr != nil {
		return define.PlaceInfo{}, true, generalErr.AppendSource("QueryPlace")
	}

	return placeInfo, true, nil
}

func (t *TripHandle) SaveTripNodes(tripIdentity string, tripNodes define.MulTripNode) *define.GeneralError {
	buf := bytes.NewBuffer(nil)
	writer := protocol.NewWriter(buf, 0)
	tripNodes.Marshal(writer)

	err := t.resHandle.SaveResource(
		ResourceTypeTripNodes,
		tripIdentity,
		buf.Bytes(),
	)
	if err != nil {
		return define.NewGeneralError("SaveTripNodes", err, define.LangKeyTripNodeSaveUnknownErr)
	}

	return nil
}

func (t *TripHandle) LoadTripNodes(tripIdentity string) (tripNodes define.MulTripNode) {
	data, found := t.resHandle.LoadResource(ResourceTypeTripNodes, tripIdentity)
	if !found {
		return tripNodes
	}

	buf := bytes.NewBuffer(data)
	reader := protocol.NewReader(buf, 0, false)
	tripNodes.Marshal(reader)

	return tripNodes
}

func (t *TripHandle) DeleteTripNodes(tripIdentity string) *define.GeneralError {
	err := t.resHandle.DeleteResource(ResourceTypeTripNodes, tripIdentity)
	if err != nil {
		return define.NewGeneralError("DeleteTripNodes", err, define.LangKeyTripNodeDeleteUnknownErr)
	}
	return nil
}

func (t *TripHandle) CreateTrip(
	tx *gorm.DB,
	userUniqueID uint32,
	tripName string,
	tripDate time.Time,
	travelMode uint8,
	tripNodes define.MulTripNode,
) (generalErr *define.GeneralError) {
	err := tx.Transaction(func(tx *gorm.DB) error {
		temp := define.TripInfo{
			UserUniqueID:   userUniqueID,
			TripName:       tripName,
			TripDate:       tripDate,
			TravelMode:     travelMode,
			TripStatus:     define.TripStatusInPlanning,
			CurrentVersion: define.TripCurrentVersionDefault,
			UpdateUnixTime: time.Now().Unix(),
		}

		result := tx.Create(&temp)
		if result.Error == nil {
			return t.SaveTripNodes(temp.TripIdentity, tripNodes)
		}
		if !errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return result.Error
		}

		return define.NewGeneralError(
			"CreateTrip",
			fmt.Errorf("Given trip %s already exists for user %d", tripName, userUniqueID),
			define.LangKeyTripCreateNameUsedErr,
		)
	})

	if err != nil {
		return define.NewGeneralError("CreateTrip", err, define.LangKeyTripCreateUnknownErr)
	}

	return nil
}

func (t *TripHandle) QueryTripByIdentity(tx *gorm.DB, tripIdentity string) (trip define.TripInfo, found bool, generalErr *define.GeneralError) {
	result := tx.
		Preload("OwnerInfo").
		Where("trip_identity = ?", tripIdentity).
		First(&trip)
	if result.Error == nil {
		return trip, true, nil
	}
	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return trip, false, define.NewGeneralError("QueryTripByIdentity", result.Error, define.LangKeyTripQueryUnknownErr)
	}
	return define.TripInfo{}, false, nil
}

func (t *TripHandle) QueryTripByOwner(tx *gorm.DB, userUniqueID uint32) (trips []define.TripInfo) {
	_ = tx.
		Preload("OwnerInfo").
		Where("user_unique_id = ?", userUniqueID).
		Order("trip_date ASC, trip_unique_id ASC").
		Find(&trips)
	return
}

func (t *TripHandle) UpdateTrip(
	tx *gorm.DB,
	tripIdentity string,
	tripUpdater func(tx *gorm.DB, trip *define.TripInfo) *define.GeneralError,
) *define.GeneralError {
	err := tx.Transaction(func(tx *gorm.DB) error {
		trip, found, generalErr := t.QueryTripByIdentity(
			tx.Clauses(clause.Locking{Strength: "UPDATE"}),
			tripIdentity,
		)
		if generalErr != nil {
			return generalErr
		}
		if !found {
			return define.NewGeneralError("UpdateTrip", fmt.Errorf("Target trip not found"), define.LangKeyTripQueryNotFoundErr)
		}

		generalErr = tripUpdater(tx, &trip)
		if generalErr != nil {
			return generalErr
		}

		result := tx.
			Where("user_unique_id = ? AND trip_name = ? AND trip_identity <> ?", trip.UserUniqueID, trip.TripName, trip.TripIdentity).
			First(&define.TripInfo{})
		if result.Error == nil {
			return define.NewGeneralError(
				"UpdateTrip",
				fmt.Errorf("Target trip name %s already used; trip.UserUniqueID = %d", trip.TripName, trip.UserUniqueID),
				define.LangKeyTripUpdateNameUsedErr,
			)
		}
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return result.Error
		}

		return tx.
			Model(&trip).
			Update("update_unix_time", time.Now().Unix()).
			Error
	})

	if err == nil {
		return nil
	}
	if generalErr, ok := err.(*define.GeneralError); ok {
		return generalErr.AppendSource("UpdateTrip")
	}

	return define.NewGeneralError("UpdateTrip", err, define.LangKeyTripUpdateUnknownErr)
}

func (t *TripHandle) DeleteTrip(tx *gorm.DB, tripIdentity string) *define.GeneralError {
	err := tx.Transaction(func(tx *gorm.DB) error {
		trip, found, generalErr := t.QueryTripByIdentity(
			tx.Clauses(clause.Locking{Strength: "UPDATE"}),
			tripIdentity,
		)
		if generalErr != nil {
			return generalErr
		}
		if !found {
			return define.NewGeneralError("", fmt.Errorf("Target trip not found"), define.LangKeyTripQueryNotFoundErr)
		}

		if err := tx.Delete(&trip).Error; err != nil {
			return err
		}
		if err := t.DeleteTripNodes(trip.TripIdentity); err != nil {
			return err
		}

		return nil
	})

	if err == nil {
		return nil
	}
	if generalErr, ok := err.(*define.GeneralError); ok {
		return generalErr.AppendSource("DeleteTrip")
	}

	return define.NewGeneralError("DeleteTrip", err, define.LangKeyTripDeleteUnknownErr)
}
