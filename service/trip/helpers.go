package trip

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/database/handle"
	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/Happy2018new/evercare-journey-backend/service/auth"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/Happy2018new/evercare-journey-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	maxTripIdentityCount     = 100
	maxTripNodeCount         = 100
	maxOptimizationNodeCount = 30
	maxPlacePage             = 1000
	maxPlacePageSize         = 25
	maxNearbyRadius          = 50000
)

func validateTripSession(session general.BasicSessionInfo, source string) *define.GeneralError {
	status, generalErr := auth.ValidateSession(session)
	if generalErr != nil {
		return generalErr.AppendSource(source)
	}
	if status != auth.ValidateSessionStatusValidSession {
		return define.NewGeneralError(
			source,
			fmt.Errorf("failed to validate current session, status=%d", status),
			define.LangKeyGeneralInvalidSession,
		)
	}
	return nil
}

func loadTripUser(session general.BasicSessionInfo, source string) (*define.UserData, *define.GeneralError) {
	if generalErr := validateTripSession(session, source); generalErr != nil {
		return nil, generalErr
	}
	user, found, generalErr := auth.LoadUser(session.UserIdentity, false)
	if generalErr != nil {
		return nil, generalErr.AppendSource(source)
	}
	if !found || user == nil {
		return nil, define.NewGeneralError(
			source,
			fmt.Errorf("authenticated user was not found"),
			define.LangKeyGeneralInvalidSession,
		)
	}
	return user, nil
}

func respondTripError(c *gin.Context, response any, source string, generalErr *define.GeneralError) {
	if generalErr == nil {
		generalErr = define.NewGeneralError(source, fmt.Errorf("unknown service error"), define.LangKeyGeneralUnknownErr)
	}
	// Every trip response embeds BasicResponseInfo. The concrete response is
	// supplied by the caller so its JSON shape remains unchanged.
	switch typed := response.(type) {
	case PlaceByIdentityResponse:
		typed.BasicResponseInfo = general.FromGeneralError(generalErr)
		c.JSON(http.StatusOK, typed)
	case QueryPlaceResponse:
		typed.BasicResponseInfo = general.FromGeneralError(generalErr)
		c.JSON(http.StatusOK, typed)
	case NearbyPlaceResponse:
		typed.BasicResponseInfo = general.FromGeneralError(generalErr)
		c.JSON(http.StatusOK, typed)
	case CreateTripResponse:
		typed.BasicResponseInfo = general.FromGeneralError(generalErr)
		c.JSON(http.StatusOK, typed)
	case QueryTripsResponse:
		typed.BasicResponseInfo = general.FromGeneralError(generalErr)
		c.JSON(http.StatusOK, typed)
	case QueryOwnedTripResponse:
		typed.BasicResponseInfo = general.FromGeneralError(generalErr)
		c.JSON(http.StatusOK, typed)
	case QueryTripVersionResponse:
		typed.BasicResponseInfo = general.FromGeneralError(generalErr)
		c.JSON(http.StatusOK, typed)
	case UpdateTripResponse:
		typed.BasicResponseInfo = general.FromGeneralError(generalErr)
		c.JSON(http.StatusOK, typed)
	case OptimizeTripResponse:
		typed.BasicResponseInfo = general.FromGeneralError(generalErr)
		c.JSON(http.StatusOK, typed)
	case EditTripNodeResponse:
		typed.BasicResponseInfo = general.FromGeneralError(generalErr)
		c.JSON(http.StatusOK, typed)
	case DeleteTripResponse:
		typed.BasicResponseInfo = general.FromGeneralError(generalErr)
		c.JSON(http.StatusOK, typed)
	default:
		c.JSON(http.StatusOK, general.FromGeneralError(generalErr))
	}
}

func invalidTripRequest(source string, format string, args ...any) *define.GeneralError {
	return invalidTripRequestWithKey(source, define.LangKeyTripRequestBodyInvalid, format, args...)
}

func invalidTripRequestWithKey(source string, key string, format string, args ...any) *define.GeneralError {
	return define.NewGeneralError(source, fmt.Errorf(format, args...), key)
}

func validateUUIDIdentity(source string, fieldName string, value string) *define.GeneralError {
	_, generalErr := canonicalUUIDIdentity(source, fieldName, value)
	return generalErr
}

func canonicalUUIDIdentity(source string, fieldName string, value string) (string, *define.GeneralError) {
	value = strings.TrimSpace(value)
	key := define.LangKeyTripIdentityInvalid
	switch {
	case strings.Contains(fieldName, "hot_place_identity"):
		key = define.LangKeyHotPlaceIdentityInvalid
	case strings.Contains(fieldName, "trip_identity"):
		key = define.LangKeyTripIdentityInvalid
	case strings.Contains(fieldName, "place_identity"):
		key = define.LangKeyPlaceIdentityInvalid
	}
	if value == "" {
		return "", invalidTripRequestWithKey(source, key, "%s cannot be empty", fieldName)
	}
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil {
		return "", invalidTripRequestWithKey(source, key, "%s must be a valid UUID", fieldName)
	}
	return parsed.String(), nil
}

func validateTripIdentityList(source string, identities []string) *define.GeneralError {
	if len(identities) == 0 || len(identities) > maxTripIdentityCount {
		return invalidTripRequestWithKey(source, define.LangKeyTripIdentityListInvalid, "trip identity count must be between 1 and %d", maxTripIdentityCount)
	}
	seen := make(map[string]struct{}, len(identities))
	for index, identity := range identities {
		canonical, generalErr := canonicalUUIDIdentity(source, fmt.Sprintf("trip_identity[%d]", index), identity)
		if generalErr != nil {
			return generalErr
		}
		identities[index] = canonical
		if _, exists := seen[canonical]; exists {
			return invalidTripRequestWithKey(source, define.LangKeyTripIdentityListInvalid, "trip_identity[%d] is duplicated", index)
		}
		seen[canonical] = struct{}{}
	}
	return nil
}

func normalizeTripName(source string, name string) (string, *define.GeneralError) {
	name = strings.TrimSpace(name)
	length := utf8.RuneCountInString(name)
	if length == 0 || length > define.TripNameMaxLengthDefault {
		return "", define.NewGeneralError(
			source,
			fmt.Errorf("trip name length is %d", length),
			define.LangKeyTripNameInvalid,
		)
	}
	return name, nil
}

func validateTripDate(source string, dateValue interface {
	IsZero() bool
	Year() int
}) *define.GeneralError {
	if dateValue.IsZero() {
		return invalidTripRequestWithKey(source, define.LangKeyTripDateInvalid, "trip_date cannot be zero")
	}
	if dateValue.Year() < 1000 || dateValue.Year() > 9999 {
		return invalidTripRequestWithKey(source, define.LangKeyTripDateInvalid, "trip_date year must be between 1000 and 9999")
	}
	return nil
}

func validateTravelMode(source string, mode uint8) *define.GeneralError {
	if mode > define.TripTravelModeTransit {
		return invalidTripRequestWithKey(source, define.LangKeyTripTravelModeInvalid, "unsupported travel_mode %d", mode)
	}
	return nil
}

func validateTripStatus(source string, status uint8) *define.GeneralError {
	if status > define.TripStatusCancelled {
		return invalidTripRequestWithKey(source, define.LangKeyTripStatusInvalid, "unsupported trip_status %d", status)
	}
	return nil
}

func validateTripStatusTransition(source string, currentStatus uint8, nextStatus uint8) *define.GeneralError {
	if currentStatus == nextStatus {
		return nil
	}
	allowed := false
	switch currentStatus {
	case define.TripStatusInPlanning:
		allowed = nextStatus == define.TripStatusInProgress || nextStatus == define.TripStatusCancelled
	case define.TripStatusInProgress:
		allowed = nextStatus == define.TripStatusCompleted || nextStatus == define.TripStatusCancelled
	case define.TripStatusCompleted, define.TripStatusCancelled:
		allowed = false
	}
	if allowed {
		return nil
	}
	return invalidTripRequestWithKey(
		source,
		define.LangKeyTripStatusTransitionInvalid,
		"trip status cannot transition from %d to %d",
		currentStatus,
		nextStatus,
	)
}

func validateTripEditable(source string, status uint8) *define.GeneralError {
	if status == define.TripStatusCompleted || status == define.TripStatusCancelled {
		return invalidTripRequestWithKey(
			source,
			define.LangKeyTripStatusTerminal,
			"trip with terminal status %d cannot be edited",
			status,
		)
	}
	return nil
}

func nextTripVersion(source string, current uint32) (uint32, *define.GeneralError) {
	if current == ^uint32(0) {
		return 0, invalidTripRequestWithKey(
			source,
			define.LangKeyTripVersionExhausted,
			"trip version has reached the maximum uint32 value",
		)
	}
	if current == 0 {
		return define.TripCurrentVersionDefault, nil
	}
	return current + 1, nil
}

func validateAmapPlaceID(source string, fieldName string, value string) (string, *define.GeneralError) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 64 {
		return "", invalidTripRequestWithKey(source, define.LangKeyPlaceAmapIdentityInvalid, "%s must contain 1-64 characters", fieldName)
	}
	return value, nil
}

func validatePage(source string, page uint32, pageSize uint32) *define.GeneralError {
	if page > maxPlacePage {
		return invalidTripRequestWithKey(source, define.LangKeyPlaceSearchPageInvalid, "page must be between 0 and %d", maxPlacePage)
	}
	if pageSize > maxPlacePageSize {
		return invalidTripRequestWithKey(source, define.LangKeyPlaceSearchPageSizeInvalid, "page_size must be between 0 and %d", maxPlacePageSize)
	}
	return nil
}

func validateCoordinate(source string, longitude float64, latitude float64) *define.GeneralError {
	if math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180 {
		return invalidTripRequestWithKey(source, define.LangKeyPlaceNearbyLongitudeInvalid, "longitude must be between -180 and 180")
	}
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90 {
		return invalidTripRequestWithKey(source, define.LangKeyPlaceNearbyLatitudeInvalid, "latitude must be between -90 and 90")
	}
	if longitude == 0 && latitude == 0 {
		return invalidTripRequestWithKey(source, define.LangKeyPlaceNearbyCoordinateInvalid, "longitude and latitude cannot both be zero")
	}
	return nil
}

func placeDataFromAmap(place utils.AmapPlace) PlaceData {
	return PlaceData{
		ProviderName:     define.PlaceProviderNameDefault,
		ProviderPlaceID:  place.ProviderPlaceID,
		Name:             place.Name,
		CategoryCode:     place.CategoryCode,
		CategoryName:     place.CategoryName,
		FullAddress:      place.FullAddress,
		ProvinceName:     place.ProvinceName,
		CityName:         place.CityName,
		DistrictName:     place.DistrictName,
		AdCode:           place.AdCode,
		Longitude:        place.Longitude,
		Latitude:         place.Latitude,
		CoordinateSystem: define.PlaceCoordinateSystemDefault,
	}
}

func placeDataFromInfo(place define.PlaceInfo) PlaceData {
	return PlaceData{
		ProviderName:     place.ProviderName,
		ProviderPlaceID:  place.ProviderPlaceID,
		Name:             place.PlaceName,
		CategoryCode:     place.CategoryCode,
		CategoryName:     place.CategoryName,
		FullAddress:      place.FullAddress,
		ProvinceName:     place.InWhichProvince,
		CityName:         place.InWhichCity,
		DistrictName:     place.InWhichDistrict,
		AdCode:           place.AdCode,
		Longitude:        place.Longitude,
		Latitude:         place.Latitude,
		CoordinateSystem: place.CoordinateSystem,
	}
}

func tripDataFromInfo(trip define.TripInfo, nodes define.MulTripNode) TripData {
	tripNodes := make([]TripNodeData, 0, len(nodes))
	for _, node := range nodes {
		noteString := ""
		if node.NoteString != uuid.Nil {
			noteString = node.NoteString.String()
		}
		tripNodes = append(tripNodes, TripNodeData{
			PlaceIdentity: node.PlaceIdentity,
			NoteString:    noteString,
		})
	}
	return TripData{
		TripIdentity:   trip.TripIdentity,
		TripName:       trip.TripName,
		TripDate:       trip.TripDate,
		TravelMode:     trip.TravelMode,
		TripStatus:     trip.TripStatus,
		CurrentVersion: trip.CurrentVersion,
		UpdateUnixTime: trip.UpdateUnixTime,
		Nodes:          tripNodes,
	}
}

func parseTripNodeNote(source string, value string) (uuid.UUID, *define.GeneralError) {
	value = strings.TrimSpace(value)
	if value == "" {
		return uuid.Nil, nil
	}
	note, err := uuid.Parse(value)
	if err != nil || note == uuid.Nil {
		return uuid.Nil, invalidTripRequestWithKey(source, define.LangKeyTripNodeNoteInvalid, "note_string must be a valid UUID")
	}
	return note, nil
}

func validateStoredTripNodes(source string, nodes define.MulTripNode) *define.GeneralError {
	if len(nodes) < 2 || len(nodes) > maxTripNodeCount {
		return define.NewGeneralError(
			source,
			fmt.Errorf("stored trip node count %d is outside the supported range", len(nodes)),
			define.LangKeyTripDataCorrupt,
		)
	}
	for index, node := range nodes {
		identity := strings.TrimSpace(node.PlaceIdentity)
		parsed, err := uuid.Parse(identity)
		if err != nil || parsed == uuid.Nil {
			return define.NewGeneralError(
				source,
				fmt.Errorf("stored node %d has invalid place identity", index),
				define.LangKeyTripDataCorrupt,
			)
		}
		// UUIDs are case-insensitive, but the database and resource keys use
		// canonical lower-case strings. Normalize the in-memory slice before it
		// is returned or passed to a place lookup.
		nodes[index].PlaceIdentity = parsed.String()
	}
	return nil
}

func ensurePlaceByAmapID(ctx context.Context, tx *gorm.DB, providerPlaceID string) (define.PlaceInfo, *define.GeneralError) {
	var generalErr *define.GeneralError
	providerPlaceID, generalErr = validateAmapPlaceID("ensurePlaceByAmapID", "amap_place_id", providerPlaceID)
	if generalErr != nil {
		return define.PlaceInfo{}, generalErr
	}
	place, found, generalErr := environment.DB.TripHandle().QueryPlace(
		ctx,
		tx,
		handle.QueryPlaceActionSearchByAmapPlaceID,
		providerPlaceID,
	)
	if generalErr != nil {
		return define.PlaceInfo{}, generalErr
	}
	if found {
		if place.PlaceStatus != define.PlaceStatusActive {
			return define.PlaceInfo{}, define.NewGeneralError(
				"ensurePlaceByAmapID",
				fmt.Errorf("place %s is not active", providerPlaceID),
				define.LangKeyPlaceQueryNotFoundErr,
			)
		}
		return place, nil
	}

	amapPlace, err := utils.GetAmapPlaceByID(ctx, providerPlaceID)
	if err != nil {
		return define.PlaceInfo{}, define.NewGeneralError(
			"ensurePlaceByAmapID",
			err,
			define.LangKeyPlaceQueryUnknownErr,
		)
	}
	if strings.TrimSpace(amapPlace.ProviderPlaceID) != providerPlaceID {
		return define.PlaceInfo{}, define.NewGeneralError(
			"ensurePlaceByAmapID",
			fmt.Errorf("Amap returned a different place identity for %s", providerPlaceID),
			define.LangKeyPlaceProviderIdentityInvalid,
		)
	}
	placeInfo := handle.FromUtilsPlace(amapPlace)
	placeInfo, generalErr = environment.DB.TripHandle().StorePlace(tx, placeInfo)
	if generalErr != nil {
		return define.PlaceInfo{}, generalErr
	}
	return placeInfo, nil
}

func loadOwnedTrip(tx *gorm.DB, userUniqueID uint32, tripIdentity string, source string) (define.TripInfo, *define.GeneralError) {
	trip, found, generalErr := environment.DB.TripHandle().QueryTripByIdentity(tx, tripIdentity)
	if generalErr != nil {
		return define.TripInfo{}, generalErr.AppendSource(source)
	}
	if !found {
		return define.TripInfo{}, define.NewGeneralError(source, fmt.Errorf("target trip not found"), define.LangKeyTripQueryNotFoundErr)
	}
	if trip.UserUniqueID != userUniqueID {
		// Keep the internal message identical to the not-found case as well. The
		// response currently includes DebugErrorInfo, so differing text would
		// disclose whether an arbitrary identity belongs to another user.
		return define.TripInfo{}, define.NewGeneralError(source, fmt.Errorf("target trip not found"), define.LangKeyTripQueryNotFoundErr)
	}
	return trip, nil
}

func validPlaceCoordinate(place define.PlaceInfo) bool {
	return !math.IsNaN(place.Longitude) && !math.IsInf(place.Longitude, 0) &&
		!math.IsNaN(place.Latitude) && !math.IsInf(place.Latitude, 0) &&
		place.Longitude >= -180 && place.Longitude <= 180 &&
		place.Latitude >= -90 && place.Latitude <= 90 &&
		!(place.Longitude == 0 && place.Latitude == 0)
}
