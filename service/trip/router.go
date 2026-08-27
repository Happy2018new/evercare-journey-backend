package trip

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/Happy2018new/evercare-journey-backend/service/general"
)

type PlaceByIdentityRequest struct {
	general.BasicSessionInfo
	PlaceIdentity string `json:"place_identity"`
}

type PlaceByIdentityResponse struct {
	general.BasicResponseInfo
	PlaceData PlaceData `json:"place_data"`
}

type QueryPlaceRequest struct {
	general.BasicSessionInfo
	Keywords  string `json:"keywords"`
	City      string `json:"city,omitempty"`
	Category  string `json:"category,omitempty"`
	CityLimit bool   `json:"city_limit,omitempty"`
	Page      uint32 `json:"page,omitempty"`
	PageSize  uint32 `json:"page_size,omitempty"`
}

type QueryPlaceResponse struct {
	general.BasicResponseInfo
	TotalCount int         `json:"total_count"`
	PlaceData  []PlaceData `json:"place_data"`
}

type NearbyPlaceRequest struct {
	general.BasicSessionInfo
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	Radius    uint32  `json:"radius,omitempty"`
	Keywords  string  `json:"keywords,omitempty"`
	Category  string  `json:"category,omitempty"`
	City      string  `json:"city,omitempty"`
	CityLimit bool    `json:"city_limit,omitempty"`
	Page      uint32  `json:"page,omitempty"`
	PageSize  uint32  `json:"page_size,omitempty"`
	SortRule  string  `json:"sort_rule,omitempty"`
}

type NearbyPlaceResponse struct {
	general.BasicResponseInfo
	TotalCount int         `json:"total_count"`
	PlaceData  []PlaceData `json:"place_data"`
}

type CreateTripRequest struct {
	general.BasicSessionInfo
	StartAmapPlaceID string    `json:"start_amap_place_id"`
	EndAmapPlaceID   string    `json:"end_amap_place_id"`
	TripName         string    `json:"trip_name"`
	TripDate         time.Time `json:"trip_date"`
	TravelMode       uint8     `json:"travel_mode"`
}

// UnmarshalJSON accepts both the mobile client's date-only form and the
// standard RFC3339 form used by time.Time. Invalid or missing values are left
// zero so the service layer can return the dedicated trip-date lang key.
func (request *CreateTripRequest) UnmarshalJSON(data []byte) error {
	type requestAlias CreateTripRequest
	var payload struct {
		*requestAlias
		TripDate json.RawMessage `json:"trip_date"`
	}
	payload.requestAlias = (*requestAlias)(request)
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	request.TripDate = parseRequestTripDate(payload.TripDate)
	return nil
}

type CreateTripResponse struct {
	general.BasicResponseInfo
	TripIdentity   string `json:"trip_identity"`
	CurrentVersion uint32 `json:"current_version"`
}

type QueryTripsRequest struct {
	general.BasicSessionInfo
	TripIdentity []string `json:"trip_identity"`
}

type QueryTripsResponse struct {
	general.BasicResponseInfo
	TripData []TripData `json:"trip_data"`
}

type QueryOwnedTripRequest struct {
	general.BasicSessionInfo
}

type QueryOwnedTripResponse struct {
	general.BasicResponseInfo
	TripIdentity []string `json:"trip_identity"`
}

type QueryTripVersionRequest struct {
	general.BasicSessionInfo
	TripIdentity []string `json:"trip_identity"`
}

type QueryTripVersionResponse struct {
	general.BasicResponseInfo
	TripVersion []TripVersionData `json:"trip_version"`
}

type UpdateTripRequest struct {
	general.BasicSessionInfo
	TripIdentity    string    `json:"trip_identity"`
	ExpectedVersion *uint32   `json:"expected_version,omitempty"`
	TripName        string    `json:"trip_name"`
	TripDate        time.Time `json:"trip_date"`
	TravelMode      uint8     `json:"travel_mode"`
	TripStatus      uint8     `json:"trip_status"`
}

func (request *UpdateTripRequest) UnmarshalJSON(data []byte) error {
	type requestAlias UpdateTripRequest
	var payload struct {
		*requestAlias
		TripDate json.RawMessage `json:"trip_date"`
	}
	payload.requestAlias = (*requestAlias)(request)
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	request.TripDate = parseRequestTripDate(payload.TripDate)
	return nil
}

func parseRequestTripDate(raw json.RawMessage) time.Time {
	if len(raw) == 0 || string(raw) == "null" {
		return time.Time{}
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return time.Time{}
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.Local)
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		local := parsed.In(time.Local)
		return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
	}
	return time.Time{}
}

type UpdateTripResponse struct {
	general.BasicResponseInfo
	TripVersion uint32 `json:"trip_version"`
}

type OptimizeTripRequest struct {
	general.BasicSessionInfo
	TripIdentity string `json:"trip_identity"`
}

type OptimizeTripResponse struct {
	general.BasicResponseInfo
	NewTripData TripData `json:"new_trip_data"`
}

const (
	EditTripNodeRequestActionAdd uint8 = iota
	EditTripNodeRequestActionDelete
	EditTripNodeRequestActionMove
	EditTripNodeRequestActionUpdate
	EditTripNodeRequestActionSetCompleted
	EditTripNodeRequestActionSetNote
)

type EditTripNodeRequest struct {
	general.BasicSessionInfo
	TripIdentity    string  `json:"trip_identity"`
	ExpectedVersion *uint32 `json:"expected_version,omitempty"`
	RequestAction   uint8   `json:"request_action"`

	NodeIndex uint8 `json:"node_index"`
	MoveToInd uint8 `json:"move_to_ind,omitempty"`

	AmapPlaceID string  `json:"amap_place_id,omitempty"`
	NoteString  *string `json:"note_string,omitempty"`
	IsCompleted *bool   `json:"is_completed,omitempty"`
}

type EditTripNodeResponse struct {
	general.BasicResponseInfo
	TripVersion uint32 `json:"trip_version"`
}

type DeleteTripRequest struct {
	general.BasicSessionInfo
	TripIdentity string `json:"trip_identity"`
}

type DeleteTripResponse struct {
	general.BasicResponseInfo
}
