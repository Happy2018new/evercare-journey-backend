package handle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
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
	var result *gorm.DB

	// Keep the caller-provided identity separate from the row selected by the
	// provider identity. Silently replacing a non-empty local identity would
	// make a stale/corrupt reference appear valid while returning a different
	// place to the caller.
	requestedPlaceIdentity := strings.TrimSpace(place.PlaceIdentity)
	place.PlaceIdentity = requestedPlaceIdentity
	place.SyncUnixTime = time.Now().Unix()
	foundExisting := false
	if place.PlaceIdentity != "" {
		result = tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("place_identity = ?", place.PlaceIdentity).
			First(&existing)
		switch {
		case result.Error == nil:
			foundExisting = true
		case !errors.Is(result.Error, gorm.ErrRecordNotFound):
			return place, define.NewGeneralError("StorePlace", result.Error, define.LangKeyPlaceQueryUnknownErr)
		}
	}
	if !foundExisting && place.ProviderName != "" && place.ProviderPlaceID != "" {
		// The provider identity is unique as well. Looking it up here avoids a
		// duplicate row when two requests fetch the same POI concurrently.
		result = tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("provider_name = ? AND provider_place_id = ?", place.ProviderName, place.ProviderPlaceID).
			First(&existing)
		switch {
		case result.Error == nil:
			foundExisting = true
		case !errors.Is(result.Error, gorm.ErrRecordNotFound):
			return place, define.NewGeneralError("StorePlace", result.Error, define.LangKeyPlaceQueryUnknownErr)
		}
	}

	if !foundExisting {
		if place.PlaceIdentity == "" {
			place.PlaceIdentity = uuid.NewString()
		}
		if err := tx.Create(&place).Error; err != nil {
			if !errors.Is(err, gorm.ErrDuplicatedKey) || place.ProviderName == "" || place.ProviderPlaceID == "" {
				return place, define.NewGeneralError("StorePlace", err, define.LangKeyPlaceSaveUnknownErr)
			}
			// The provider lookup and insert are necessarily separate operations.
			// A concurrent request may win the unique key between them; reuse the
			// row it created instead of turning that normal race into an API error.
			result = tx.
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("provider_name = ? AND provider_place_id = ?", place.ProviderName, place.ProviderPlaceID).
				First(&existing)
			if result.Error != nil {
				return place, define.NewGeneralError("StorePlace", result.Error, define.LangKeyPlaceSaveUnknownErr)
			}
			foundExisting = true
		}
		if !foundExisting {
			return place, nil
		}
	}

	if place.ProviderName != "" && place.ProviderPlaceID != "" &&
		(existing.ProviderName != place.ProviderName || existing.ProviderPlaceID != place.ProviderPlaceID) {
		return place, define.NewGeneralError(
			"StorePlace",
			fmt.Errorf("place identity %s does not match provider identity %s/%s", existing.PlaceIdentity, place.ProviderName, place.ProviderPlaceID),
			define.LangKeyPlaceProviderIdentityInvalid,
		)
	}
	if requestedPlaceIdentity != "" && !strings.EqualFold(requestedPlaceIdentity, existing.PlaceIdentity) {
		return place, define.NewGeneralError(
			"StorePlace",
			fmt.Errorf("requested place identity %s resolves to %s", requestedPlaceIdentity, existing.PlaceIdentity),
			define.LangKeyPlaceProviderIdentityInvalid,
		)
	}

	place.PlaceIdentity = existing.PlaceIdentity
	place.PlaceUniqueID = existing.PlaceUniqueID
	place.ProviderName = existing.ProviderName
	place.ProviderPlaceID = existing.ProviderPlaceID
	// Provider detail responses can omit optional fields. Do not erase a
	// previously known value merely because one refresh response was partial.
	if strings.TrimSpace(place.PlaceName) == "" {
		place.PlaceName = existing.PlaceName
	}
	if strings.TrimSpace(place.CategoryCode) == "" {
		place.CategoryCode = existing.CategoryCode
	}
	if strings.TrimSpace(place.CategoryName) == "" {
		place.CategoryName = existing.CategoryName
	}
	if strings.TrimSpace(place.FullAddress) == "" {
		place.FullAddress = existing.FullAddress
	}
	if strings.TrimSpace(place.InWhichProvince) == "" {
		place.InWhichProvince = existing.InWhichProvince
	}
	if strings.TrimSpace(place.InWhichCity) == "" {
		place.InWhichCity = existing.InWhichCity
	}
	if strings.TrimSpace(place.InWhichDistrict) == "" {
		place.InWhichDistrict = existing.InWhichDistrict
	}
	if strings.TrimSpace(place.AdCode) == "" {
		place.AdCode = existing.AdCode
	}
	if strings.TrimSpace(place.CoordinateSystem) == "" {
		place.CoordinateSystem = existing.CoordinateSystem
	}
	if place.Longitude == 0 && place.Latitude == 0 {
		place.Longitude = existing.Longitude
		place.Latitude = existing.Latitude
	}
	if existing.PlaceStatus != define.PlaceStatusActive {
		place.PlaceStatus = existing.PlaceStatus
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
	if result.RowsAffected == 0 {
		// MySQL can report zero rows for a valid no-op update. Confirm that the
		// row still exists before allowing callers to persist a reference to it.
		var current define.PlaceInfo
		check := tx.Where("place_identity = ?", existing.PlaceIdentity).First(&current)
		if errors.Is(check.Error, gorm.ErrRecordNotFound) {
			return place, define.NewGeneralError("StorePlace", fmt.Errorf("place row %s disappeared during update", existing.PlaceIdentity), define.LangKeyPlaceSaveUnknownErr)
		}
		if check.Error != nil {
			return place, define.NewGeneralError("StorePlace", check.Error, define.LangKeyPlaceSaveUnknownErr)
		}
	}

	return place, nil
}

func (t *TripHandle) QueryPlace(
	ctx context.Context,
	tx *gorm.DB,
	action uint8,
	keyword any,
) (place define.PlaceInfo, found bool, generalErr *define.GeneralError) {
	var query *gorm.DB
	if ctx == nil {
		ctx = context.Background()
	}
	tx = tx.WithContext(ctx)

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
	// An unavailable or archived place is an explicit local decision. A
	// normal read must not silently reactivate it from a provider response.
	if place.PlaceStatus != define.PlaceStatusActive {
		return place, true, nil
	}
	if place.SyncUnixTime > 0 && time.Now().Unix() <= place.SyncUnixTime+PlaceRefreshDefaultTime {
		return place, true, nil
	}

	amapPlace, err := utils.GetAmapPlaceByID(ctx, place.ProviderPlaceID)
	if err != nil {
		return place, false, define.NewGeneralError(
			"QueryPlace",
			fmt.Errorf("Failed to refresh place %s due to %w", place.PlaceIdentity, err),
			define.LangKeyPlaceRefreshUnknownErr,
		)
	}
	if strings.TrimSpace(amapPlace.ProviderPlaceID) != strings.TrimSpace(place.ProviderPlaceID) {
		return place, false, define.NewGeneralError(
			"QueryPlace",
			fmt.Errorf("provider returned identity %q for stored place %q", amapPlace.ProviderPlaceID, place.ProviderPlaceID),
			define.LangKeyPlaceProviderIdentityInvalid,
		)
	}
	placeInfo := FromUtilsPlace(amapPlace)
	placeInfo.PlaceIdentity = place.PlaceIdentity
	placeInfo.PlaceUniqueID = place.PlaceUniqueID
	placeInfo.PlaceStatus = place.PlaceStatus
	placeInfo, generalErr = t.StorePlace(tx, placeInfo)
	if generalErr != nil {
		return place, false, generalErr.AppendSource("QueryPlace")
	}

	return placeInfo, true, nil
}

func (t *TripHandle) SaveTripNodes(tripIdentity string, tripNodes define.MulTripNode) *define.GeneralError {
	if len(tripNodes) > 255 {
		return define.NewGeneralError(
			"SaveTripNodes",
			fmt.Errorf("trip node count %d exceeds serialization limit", len(tripNodes)),
			define.LangKeyTripNodeCountInvalid,
		)
	}
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
	tripNodes, _, _ = t.LoadTripNodesWithError(tripIdentity)
	return tripNodes
}

// LoadTripNodesWithError distinguishes a missing resource from a malformed
// resource. The old LoadTripNodes method is kept for callers outside the
// service layer; new request handlers should use this method.
func (t *TripHandle) LoadTripNodesWithError(tripIdentity string) (tripNodes define.MulTripNode, found bool, generalErr *define.GeneralError) {
	data, found, err := t.resHandle.LoadResourceWithError(ResourceTypeTripNodes, tripIdentity)
	if err != nil {
		return nil, false, define.NewGeneralError("LoadTripNodes", err, define.LangKeyTripDataCorrupt)
	}
	if !found {
		return nil, false, nil
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			tripNodes = nil
			generalErr = define.NewGeneralError(
				"LoadTripNodes",
				fmt.Errorf("failed to decode trip nodes: %v", recovered),
				define.LangKeyTripDataCorrupt,
			)
		}
	}()

	buf := bytes.NewBuffer(data)
	reader := protocol.NewReader(buf, 0, false)
	tripNodes.Marshal(reader)
	if buf.Len() != 0 {
		return nil, true, define.NewGeneralError(
			"LoadTripNodes",
			fmt.Errorf("decoded trip node resource contains %d trailing bytes", buf.Len()),
			define.LangKeyTripDataCorrupt,
		)
	}
	if len(tripNodes) > 255 {
		return nil, true, define.NewGeneralError(
			"LoadTripNodes",
			fmt.Errorf("decoded trip node count %d exceeds serialization limit", len(tripNodes)),
			define.LangKeyTripDataCorrupt,
		)
	}
	return tripNodes, true, nil
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
	_, generalErr = t.CreateTripWithInfo(tx, userUniqueID, tripName, tripDate, travelMode, tripNodes)
	return generalErr
}

func (t *TripHandle) CreateTripWithInfo(
	tx *gorm.DB,
	userUniqueID uint32,
	tripName string,
	tripDate time.Time,
	travelMode uint8,
	tripNodes define.MulTripNode,
) (trip define.TripInfo, generalErr *define.GeneralError) {
	return t.CreateTripWithStatus(
		tx,
		userUniqueID,
		tripName,
		tripDate,
		travelMode,
		define.TripStatusInPlanning,
		tripNodes,
	)
}

func (t *TripHandle) CreateTripWithStatus(
	tx *gorm.DB,
	userUniqueID uint32,
	tripName string,
	tripDate time.Time,
	travelMode uint8,
	tripStatus uint8,
	tripNodes define.MulTripNode,
) (trip define.TripInfo, generalErr *define.GeneralError) {
	err := tx.Transaction(func(tx *gorm.DB) error {
		var existing define.TripInfo
		nameResult := tx.
			Where("user_unique_id = ? AND trip_name = ?", userUniqueID, tripName).
			First(&existing)
		if nameResult.Error == nil {
			return define.NewGeneralError(
				"CreateTrip",
				fmt.Errorf("Given trip %s already exists for user %d", tripName, userUniqueID),
				define.LangKeyTripCreateNameUsedErr,
			)
		}
		if !errors.Is(nameResult.Error, gorm.ErrRecordNotFound) {
			return nameResult.Error
		}

		temp := define.TripInfo{
			TripIdentity:   uuid.NewString(),
			UserUniqueID:   userUniqueID,
			TripName:       tripName,
			TripDate:       tripDate,
			TravelMode:     travelMode,
			TripStatus:     tripStatus,
			CurrentVersion: define.TripCurrentVersionDefault,
			UpdateUnixTime: time.Now().Unix(),
		}

		result := tx.Create(&temp)
		if result.Error == nil {
			trip = temp
			if err := t.SaveTripNodes(temp.TripIdentity, tripNodes); err != nil {
				return err
			}
			return nil
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
		if trip.TripIdentity != "" {
			// SQL transactions cannot roll back bbolt writes. Remove the node
			// resource when the SQL transaction fails to avoid orphaned data.
			_ = t.DeleteTripNodes(trip.TripIdentity)
		}
		if generalErr, ok := err.(*define.GeneralError); ok {
			return define.TripInfo{}, generalErr.AppendSource("CreateTrip")
		}
		return define.TripInfo{}, define.NewGeneralError("CreateTrip", err, define.LangKeyTripCreateUnknownErr)
	}

	return trip, nil
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

func (t *TripHandle) QueryTripByOwnerAndName(
	tx *gorm.DB,
	userUniqueID uint32,
	tripName string,
) (trip define.TripInfo, found bool, generalErr *define.GeneralError) {
	result := tx.
		Preload("OwnerInfo").
		Where("user_unique_id = ? AND trip_name = ?", userUniqueID, tripName).
		First(&trip)
	if result.Error == nil {
		return trip, true, nil
	}
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return trip, false, nil
	}
	return trip, false, define.NewGeneralError("QueryTripByOwnerAndName", result.Error, define.LangKeyTripQueryUnknownErr)
}

func (t *TripHandle) QueryTripByOwnerWithError(
	tx *gorm.DB,
	userUniqueID uint32,
) (trips []define.TripInfo, generalErr *define.GeneralError) {
	result := tx.
		Preload("OwnerInfo").
		Where("user_unique_id = ?", userUniqueID).
		Order("trip_date ASC, trip_unique_id ASC").
		Find(&trips)
	if result.Error != nil {
		return nil, define.NewGeneralError("QueryTripByOwner", result.Error, define.LangKeyTripQueryUnknownErr)
	}
	return trips, nil
}

func (t *TripHandle) QueryTripByOwner(tx *gorm.DB, userUniqueID uint32) (trips []define.TripInfo) {
	trips, _ = t.QueryTripByOwnerWithError(tx, userUniqueID)
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

		if trip.CurrentVersion == ^uint32(0) {
			return define.NewGeneralError(
				"UpdateTrip",
				fmt.Errorf("trip version has reached the maximum uint32 value"),
				define.LangKeyTripVersionExhausted,
			)
		}
		if trip.CurrentVersion == 0 {
			trip.CurrentVersion = define.TripCurrentVersionDefault
		} else {
			trip.CurrentVersion++
		}

		result = tx.
			Model(&trip).
			Updates(map[string]any{
				"trip_name":        trip.TripName,
				"trip_date":        trip.TripDate,
				"travel_mode":      trip.TravelMode,
				"trip_status":      trip.TripStatus,
				"current_version":  trip.CurrentVersion,
				"update_unix_time": time.Now().Unix(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("trip row was not updated")
		}
		return nil
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
	// Keep the bbolt resource intact until the SQL transaction has committed.
	// Otherwise a later SQL rollback can leave a live trip without its nodes.
	deletedTripIdentity := tripIdentity
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

		result := tx.Delete(&trip)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("trip row was not deleted")
		}
		deletedTripIdentity = trip.TripIdentity
		return nil
	})

	if err != nil {
		if generalErr, ok := err.(*define.GeneralError); ok {
			return generalErr.AppendSource("DeleteTrip")
		}
		return define.NewGeneralError("DeleteTrip", err, define.LangKeyTripDeleteUnknownErr)
	}

	if generalErr := t.DeleteTripNodes(deletedTripIdentity); generalErr != nil {
		return generalErr.AppendSource("DeleteTrip")
	}
	return nil
}
