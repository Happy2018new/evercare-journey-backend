package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var AmapServiceAccessToken = "AMAP_SERVICE_API_KEY"

const (
	AmapPlaceSearchEndpoint = "https://restapi.amap.com/v3/place/text"
	AmapPlaceDetailEndpoint = "https://restapi.amap.com/v3/place/detail"
	AmapDistanceEndpoint    = "https://restapi.amap.com/v3/distance"
)

const (
	AmapDistanceTypeStraight uint8 = 0
	AmapDistanceTypeDriving  uint8 = 1
	AmapDistanceTypeWalking  uint8 = 3
)

const (
	amapRequestTimeout = time.Second * 10
	amapMaxBodySize    = 4 * 1024 * 1024
	amapMaxOrigins     = 100
)

var amapHTTPClient = &http.Client{Timeout: amapRequestTimeout}

// AmapCoordinate is a GCJ-02 coordinate. High-level callers should not pass
// WGS-84 coordinates without converting them first.
type AmapCoordinate struct {
	Longitude float64
	Latitude  float64
}

type AmapPlaceSearchOptions struct {
	Keywords  string
	City      string
	CityLimit bool
	Page      int
	PageSize  int
}

type AmapPlace struct {
	ProviderPlaceID string
	Name            string
	CategoryCode    string
	CategoryName    string
	FullAddress     string
	ProvinceName    string
	CityName        string
	DistrictName    string
	AdCode          string
	Longitude       float64
	Latitude        float64
}

type AmapPlaceSearchResult struct {
	Count  int
	Places []AmapPlace
}

type AmapDistance struct {
	OriginIndex int
	Distance    int64
	Duration    int64
}

// AmapAPIError represents an error returned by Amap rather than an HTTP or
// response-decoding failure.
type AmapAPIError struct {
	ErrorInfo string
	ErrorCode string
}

func (e *AmapAPIError) Error() string {
	return fmt.Sprintf("Amap API request failed: %s (code %s)", e.ErrorInfo, e.ErrorCode)
}

// SearchAmapPlaces searches POIs by keyword. Page and PageSize default to 1
// and 20 respectively when they are zero.
func SearchAmapPlaces(ctx context.Context, options AmapPlaceSearchOptions) (AmapPlaceSearchResult, error) {
	keywords := strings.TrimSpace(options.Keywords)
	if keywords == "" {
		return AmapPlaceSearchResult{}, fmt.Errorf("SearchAmapPlaces: Keywords cannot be empty")
	}

	page := options.Page
	if page == 0 {
		page = 1
	}
	if page < 1 {
		return AmapPlaceSearchResult{}, fmt.Errorf("SearchAmapPlaces: Page must be greater than zero")
	}

	pageSize := options.PageSize
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize < 1 || pageSize > 25 {
		return AmapPlaceSearchResult{}, fmt.Errorf("SearchAmapPlaces: Page size must be between 1 and 25")
	}

	query := url.Values{
		"keywords":  {keywords},
		"page":      {strconv.Itoa(page)},
		"offset":    {strconv.Itoa(pageSize)},
		"citylimit": {strconv.FormatBool(options.CityLimit)},
	}
	if city := strings.TrimSpace(options.City); city != "" {
		query.Set("city", city)
	}

	response, err := requestAmapJSON[amapPlaceResponse](ctx, AmapPlaceSearchEndpoint, query)
	if err != nil {
		return AmapPlaceSearchResult{}, fmt.Errorf("SearchAmapPlaces: %w", err)
	}
	count, err := parseOptionalInt(response.Count)
	if err != nil {
		return AmapPlaceSearchResult{}, fmt.Errorf("SearchAmapPlaces: Invalid result count: %w", err)
	}
	places, err := convertAmapPlaces(response.POIs)
	if err != nil {
		return AmapPlaceSearchResult{}, fmt.Errorf("SearchAmapPlaces: %w", err)
	}

	return AmapPlaceSearchResult{Count: count, Places: places}, nil
}

// GetAmapPlaceByID returns one POI by its Amap ID.
func GetAmapPlaceByID(ctx context.Context, providerPlaceID string) (AmapPlace, error) {
	providerPlaceID = strings.TrimSpace(providerPlaceID)
	if providerPlaceID == "" {
		return AmapPlace{}, fmt.Errorf("GetAmapPlaceByID: Provider place ID cannot be empty")
	}

	query := url.Values{
		"id":         {providerPlaceID},
		"extensions": {"all"},
	}
	response, err := requestAmapJSON[amapPlaceResponse](ctx, AmapPlaceDetailEndpoint, query)
	if err != nil {
		return AmapPlace{}, fmt.Errorf("GetAmapPlaceByID: %w", err)
	}

	if len(response.POIs) == 0 {
		return AmapPlace{}, fmt.Errorf("GetAmapPlaceByID: Place not found")
	}
	place, err := convertAmapPlace(response.POIs[0])
	if err != nil {
		return AmapPlace{}, fmt.Errorf("GetAmapPlaceByID: %w", err)
	}
	return place, nil
}

// QueryAmapDistances returns distance in metres and duration in seconds from
// each origin to one destination. Results use zero-based OriginIndex values.
func QueryAmapDistances(
	ctx context.Context,
	origins []AmapCoordinate,
	destination AmapCoordinate,
	distanceType uint8,
) ([]AmapDistance, error) {
	if len(origins) == 0 || len(origins) > amapMaxOrigins {
		return nil, fmt.Errorf("QueryAmapDistances: origins count must be between 1 and %d", amapMaxOrigins)
	}
	if distanceType != AmapDistanceTypeStraight &&
		distanceType != AmapDistanceTypeDriving &&
		distanceType != AmapDistanceTypeWalking {
		return nil, fmt.Errorf("QueryAmapDistances: distance type must be straight, driving, or walking")
	}

	encodedOrigins := make([]string, len(origins))
	for index, origin := range origins {
		encodedOrigins[index] = formatAmapCoordinate(origin)
	}

	query := url.Values{
		"origins":     {strings.Join(encodedOrigins, "|")},
		"destination": {formatAmapCoordinate(destination)},
		"type":        {strconv.FormatUint(uint64(distanceType), 10)},
	}
	response, err := requestAmapJSON[amapDistanceResponse](ctx, AmapDistanceEndpoint, query)
	if err != nil {
		return nil, fmt.Errorf("QueryAmapDistances: %w", err)
	}

	distances := make([]AmapDistance, 0, len(response.Results))
	for resultIndex, raw := range response.Results {
		originIndex, err := parseRequiredInt(raw.OriginID)
		if err != nil {
			return nil, fmt.Errorf("QueryAmapDistances: invalid origin ID at result %d: %w", resultIndex, err)
		}
		if originIndex < 1 || originIndex > len(origins) {
			return nil, fmt.Errorf("QueryAmapDistances: origin ID %d at result %d is out of range", originIndex, resultIndex)
		}
		distance, err := parseRequiredInt64(raw.Distance)
		if err != nil {
			return nil, fmt.Errorf("QueryAmapDistances: invalid distance at result %d: %w", resultIndex, err)
		}
		duration, err := parseOptionalInt64(raw.Duration)
		if err != nil {
			return nil, fmt.Errorf("QueryAmapDistances: invalid duration at result %d: %w", resultIndex, err)
		}
		distances = append(distances, AmapDistance{
			OriginIndex: originIndex - 1,
			Distance:    distance,
			Duration:    duration,
		})
	}
	return distances, nil
}

type amapBaseResponse struct {
	Status   string `json:"status"`
	Info     string `json:"info"`
	InfoCode string `json:"infocode"`
}

type amapPlaceResponse struct {
	amapBaseResponse
	Count flexibleString `json:"count"`
	POIs  []amapRawPlace `json:"pois"`
}

type amapRawPlace struct {
	ID       flexibleString `json:"id"`
	Name     flexibleString `json:"name"`
	Type     flexibleString `json:"type"`
	TypeCode flexibleString `json:"typecode"`
	Address  flexibleString `json:"address"`
	Location flexibleString `json:"location"`
	PName    flexibleString `json:"pname"`
	CityName flexibleString `json:"cityname"`
	ADName   flexibleString `json:"adname"`
	ADCode   flexibleString `json:"adcode"`
}

type amapDistanceResponse struct {
	amapBaseResponse
	Results []amapRawDistance `json:"results"`
}

type amapRawDistance struct {
	OriginID flexibleString `json:"origin_id"`
	Distance flexibleString `json:"distance"`
	Duration flexibleString `json:"duration"`
}

type flexibleString string

func (value *flexibleString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) || bytes.Equal(data, []byte("[]")) || bytes.Equal(data, []byte("{}")) {
		*value = ""
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*value = flexibleString(text)
		return nil
	}

	var texts []string
	if err := json.Unmarshal(data, &texts); err == nil {
		*value = flexibleString(strings.Join(texts, ", "))
		return nil
	}

	return fmt.Errorf("expected a string or string array")
}

func requestAmapJSON[T any](ctx context.Context, endpoint string, query url.Values) (result T, err error) {
	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return result, fmt.Errorf("requestAmapJSON: Invalid endpoint: %w", err)
	}

	values := requestURL.Query()
	for key, value := range query {
		for _, val := range value {
			values.Add(key, val)
		}
	}
	values.Set("key", AmapServiceAccessToken)
	values.Set("output", "JSON")
	requestURL.RawQuery = values.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return result, fmt.Errorf("requestAmapJSON: Create request: %w", err)
	}
	response, err := amapHTTPClient.Do(request)
	if err != nil {
		return result, fmt.Errorf("requestAmapJSON: Send request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return result, fmt.Errorf("requestAmapJSON: Unexpected HTTP status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, amapMaxBodySize+1))
	if err != nil {
		return result, fmt.Errorf("requestAmapJSON: Read response: %w", err)
	}
	if len(body) > amapMaxBodySize {
		return result, fmt.Errorf("requestAmapJSON: Response exceeds %d bytes", amapMaxBodySize)
	}

	var base amapBaseResponse
	if err := json.Unmarshal(body, &base); err != nil {
		return result, fmt.Errorf("requestAmapJSON: Decode response: %w", err)
	}
	if base.Status != "1" {
		return result, &AmapAPIError{ErrorInfo: base.Info, ErrorCode: base.InfoCode}
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("requestAmapJSON: Decode result: %w", err)
	}
	return result, nil
}

func convertAmapPlaces(rawPlaces []amapRawPlace) ([]AmapPlace, error) {
	places := make([]AmapPlace, 0, len(rawPlaces))
	for index, raw := range rawPlaces {
		place, err := convertAmapPlace(raw)
		if err != nil {
			return nil, fmt.Errorf("convertAmapPlaces: Invalid place at index %d: %w", index, err)
		}
		places = append(places, place)
	}
	return places, nil
}

func convertAmapPlace(raw amapRawPlace) (AmapPlace, error) {
	coordinate, err := parseAmapCoordinate(string(raw.Location))
	if err != nil {
		return AmapPlace{}, fmt.Errorf("convertAmapPlace: Invalid location for place %q due to %w", raw.ID, err)
	}
	return AmapPlace{
		ProviderPlaceID: string(raw.ID),
		Name:            string(raw.Name),
		CategoryCode:    string(raw.TypeCode),
		CategoryName:    string(raw.Type),
		FullAddress:     string(raw.Address),
		ProvinceName:    string(raw.PName),
		CityName:        string(raw.CityName),
		DistrictName:    string(raw.ADName),
		AdCode:          string(raw.ADCode),
		Longitude:       coordinate.Longitude,
		Latitude:        coordinate.Latitude,
	}, nil
}

func parseAmapCoordinate(location string) (AmapCoordinate, error) {
	parts := strings.Split(location, ",")
	if len(parts) != 2 {
		return AmapCoordinate{}, fmt.Errorf("parseAmapCoordinate: Invalid location: %s", location)
	}

	longitude, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return AmapCoordinate{}, fmt.Errorf("parseAmapCoordinate: Invalid longitude: %w", err)
	}
	latitude, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return AmapCoordinate{}, fmt.Errorf("parseAmapCoordinate: Invalid latitude: %w", err)
	}

	coordinate := AmapCoordinate{Longitude: longitude, Latitude: latitude}
	return coordinate, nil
}

func formatAmapCoordinate(coordinate AmapCoordinate) string {
	return "" +
		strconv.FormatFloat(coordinate.Longitude, 'f', 7, 64) + "," +
		strconv.FormatFloat(coordinate.Latitude, 'f', 7, 64)
}

func parseOptionalInt(value flexibleString) (int, error) {
	if value == "" {
		return 0, nil
	}

	result, err := strconv.ParseInt(string(value), 10, 0)
	if err != nil {
		return 0, fmt.Errorf("parseOptionalInt: %w", err)
	}

	return int(result), nil
}

func parseRequiredInt(value flexibleString) (int, error) {
	if value == "" {
		return 0, fmt.Errorf("parseRequiredInt: Given value is empty")
	}

	result, err := strconv.ParseInt(string(value), 10, 0)
	if err != nil {
		return 0, fmt.Errorf("parseRequiredInt: %w", err)
	}

	return int(result), nil
}

func parseOptionalInt64(value flexibleString) (int64, error) {
	if value == "" {
		return 0, nil
	}

	result, err := strconv.ParseInt(string(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parseOptionalInt64: %w", err)
	}

	return result, nil
}

func parseRequiredInt64(value flexibleString) (int64, error) {
	if value == "" {
		return 0, fmt.Errorf("parseRequiredInt64: Given value is empty")
	}

	result, err := strconv.ParseInt(string(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parseRequiredInt64: %w", err)
	}

	return result, nil
}
