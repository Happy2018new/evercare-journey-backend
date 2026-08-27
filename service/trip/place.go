package trip

import (
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/database/handle"
	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/Happy2018new/evercare-journey-backend/utils"
	"github.com/gin-gonic/gin"
)

type PlaceData struct {
	ProviderName     string  `json:"provider_name"`
	ProviderPlaceID  string  `json:"provider_place_id"`
	Name             string  `json:"name"`
	CategoryCode     string  `json:"category_code"`
	CategoryName     string  `json:"category_name"`
	FullAddress      string  `json:"full_address"`
	ProvinceName     string  `json:"province_name"`
	CityName         string  `json:"city_name"`
	DistrictName     string  `json:"district_name"`
	AdCode           string  `json:"ad_code"`
	Longitude        float64 `json:"longitude"`
	Latitude         float64 `json:"latitude"`
	CoordinateSystem string  `json:"coordinate_system"`
}

func HandlePlaceByIdentity(c *gin.Context) {
	const source = "HandlePlaceByIdentity"
	var request PlaceByIdentityRequest
	if err := c.ShouldBind(&request); err != nil {
		respondTripError(c, PlaceByIdentityResponse{}, source, define.NewGeneralError(source, err, define.LangKeyPlaceRequestBodyInvalid))
		return
	}
	generalErr := validateTripSession(request.BasicSessionInfo, source)
	if generalErr != nil {
		respondTripError(c, PlaceByIdentityResponse{}, source, generalErr)
		return
	}
	placeIdentity, generalErr := canonicalUUIDIdentity(source, "place_identity", request.PlaceIdentity)
	if generalErr != nil {
		respondTripError(c, PlaceByIdentityResponse{}, source, generalErr)
		return
	}

	place, found, generalErr := environment.DB.TripHandle().QueryPlace(
		c.Request.Context(),
		environment.DB.Database(),
		handle.QueryPlaceActionSearchByIdentity,
		placeIdentity,
	)
	if generalErr != nil {
		respondTripError(c, PlaceByIdentityResponse{}, source, generalErr)
		return
	}
	if !found || place.PlaceStatus != define.PlaceStatusActive {
		respondTripError(c, PlaceByIdentityResponse{}, source, define.NewGeneralError(source, fmt.Errorf("target place not found"), define.LangKeyPlaceQueryNotFoundErr))
		return
	}
	c.JSON(http.StatusOK, PlaceByIdentityResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		PlaceData:         placeDataFromInfo(place),
	})
}

func HandleQueryPlace(c *gin.Context) {
	const source = "HandleQueryPlace"
	var request QueryPlaceRequest
	if err := c.ShouldBind(&request); err != nil {
		respondTripError(c, QueryPlaceResponse{}, source, define.NewGeneralError(source, err, define.LangKeyPlaceRequestBodyInvalid))
		return
	}
	if generalErr := validateTripSession(request.BasicSessionInfo, source); generalErr != nil {
		respondTripError(c, QueryPlaceResponse{}, source, generalErr)
		return
	}
	request.Keywords = strings.TrimSpace(request.Keywords)
	if request.Keywords == "" || utf8.RuneCountInString(request.Keywords) > 128 {
		respondTripError(c, QueryPlaceResponse{}, source, invalidTripRequestWithKey(source, define.LangKeyPlaceSearchKeywordInvalid, "keywords must contain 1-128 characters"))
		return
	}
	if generalErr := validatePlaceSearchFilters(source, request.City, request.Category, request.Page, request.PageSize); generalErr != nil {
		respondTripError(c, QueryPlaceResponse{}, source, generalErr)
		return
	}

	result, err := utils.SearchAmapPlaces(c.Request.Context(), utils.AmapPlaceSearchOptions{
		Keywords:  request.Keywords,
		City:      strings.TrimSpace(request.City),
		Category:  strings.TrimSpace(request.Category),
		CityLimit: request.CityLimit,
		Page:      int(request.Page),
		PageSize:  int(request.PageSize),
	})
	if err != nil {
		respondTripError(c, QueryPlaceResponse{}, source, define.NewGeneralError(source, err, define.LangKeyPlaceSearchUnknownErr))
		return
	}
	placeData := make([]PlaceData, 0, len(result.Places))
	for _, place := range result.Places {
		placeData = append(placeData, placeDataFromAmap(place))
	}
	c.JSON(http.StatusOK, QueryPlaceResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		TotalCount:        result.Count,
		PlaceData:         placeData,
	})
}

func HandleNearbyPlace(c *gin.Context) {
	const source = "HandleNearbyPlace"
	var request NearbyPlaceRequest
	if err := c.ShouldBind(&request); err != nil {
		respondTripError(c, NearbyPlaceResponse{}, source, define.NewGeneralError(source, err, define.LangKeyPlaceRequestBodyInvalid))
		return
	}
	if generalErr := validateTripSession(request.BasicSessionInfo, source); generalErr != nil {
		respondTripError(c, NearbyPlaceResponse{}, source, generalErr)
		return
	}
	if generalErr := validateCoordinate(source, request.Longitude, request.Latitude); generalErr != nil {
		respondTripError(c, NearbyPlaceResponse{}, source, generalErr)
		return
	}
	if request.Radius > maxNearbyRadius {
		respondTripError(c, NearbyPlaceResponse{}, source, invalidTripRequestWithKey(source, define.LangKeyPlaceNearbyRadiusInvalid, "radius must be between 0 and %d metres", maxNearbyRadius))
		return
	}
	if generalErr := validatePlaceSearchFilters(source, request.City, request.Category, request.Page, request.PageSize); generalErr != nil {
		respondTripError(c, NearbyPlaceResponse{}, source, generalErr)
		return
	}
	request.Keywords = strings.TrimSpace(request.Keywords)
	if utf8.RuneCountInString(request.Keywords) > 128 {
		respondTripError(c, NearbyPlaceResponse{}, source, invalidTripRequestWithKey(source, define.LangKeyPlaceNearbyKeywordInvalid, "keywords cannot exceed 128 characters"))
		return
	}
	request.SortRule = strings.TrimSpace(request.SortRule)
	if request.SortRule != "" && request.SortRule != "distance" && request.SortRule != "weight" {
		respondTripError(c, NearbyPlaceResponse{}, source, invalidTripRequestWithKey(source, define.LangKeyPlaceNearbySortRuleInvalid, "sort_rule must be distance or weight"))
		return
	}

	result, err := utils.SearchAmapNearbyPlaces(c.Request.Context(), utils.AmapNearbyPlaceSearchOptions{
		Longitude: request.Longitude,
		Latitude:  request.Latitude,
		Radius:    int(request.Radius),
		Keywords:  request.Keywords,
		Category:  strings.TrimSpace(request.Category),
		City:      strings.TrimSpace(request.City),
		CityLimit: request.CityLimit,
		Page:      int(request.Page),
		PageSize:  int(request.PageSize),
		SortRule:  request.SortRule,
	})
	if err != nil {
		respondTripError(c, NearbyPlaceResponse{}, source, define.NewGeneralError(source, err, define.LangKeyPlaceNearbyQueryErr))
		return
	}
	placeData := make([]PlaceData, 0, len(result.Places))
	for _, place := range result.Places {
		placeData = append(placeData, placeDataFromAmap(place))
	}
	c.JSON(http.StatusOK, NearbyPlaceResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		TotalCount:        result.Count,
		PlaceData:         placeData,
	})
}

func validatePlaceSearchFilters(source string, city string, category string, page uint32, pageSize uint32) *define.GeneralError {
	if utf8.RuneCountInString(strings.TrimSpace(city)) > 64 {
		return invalidTripRequestWithKey(source, define.LangKeyPlaceSearchCityInvalid, "city cannot exceed 64 characters")
	}
	if utf8.RuneCountInString(strings.TrimSpace(category)) > 64 {
		return invalidTripRequestWithKey(source, define.LangKeyPlaceSearchCategoryInvalid, "category cannot exceed 64 characters")
	}
	return validatePage(source, page, pageSize)
}
