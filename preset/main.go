// The preset command initializes the application's tables and imports the
// bundled nationwide hot-place pack. It only reads the embedded JSON and image
// assets: it never calls Amap or another map-provider HTTP service. Refresh
// Amap data separately with `go run ./preset/collector -amap` when needed.
// The default res.db location is the repository root, regardless of the
// caller's current working directory:
//
//	go run ./preset
package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"log"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/database/handle"
	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	imageWidth  = 1920
	imageHeight = 1080
)

// presetAssets keeps the preset self-contained, so the command does not
// depend on the caller's current working directory.
//
//go:embed hot_places.json place_catalog.json image_sources.json images/*.jpg
var presetAssets embed.FS

type hotPlace struct {
	HotPlaceUniqueID uint32 `json:"HotPlaceUniqueID"`
	HotPlaceIdentity string `json:"HotPlaceIdentity"`
	RecommendTitle   string `json:"RecommendTitle"`
	RecommandDetail  string `json:"RecommandDetail"`
	PlaceImageItemID string `json:"PlaceImageItemID"`
	PlaceIdentity    string `json:"PlaceIdentity"`
}

type placeCatalogItem struct {
	PlaceIdentity   string  `json:"place_identity"`
	ProviderName    string  `json:"provider_name"`
	ProviderPlaceID string  `json:"provider_place_id"`
	Name            string  `json:"name"`
	CategoryCode    string  `json:"category_code"`
	CategoryName    string  `json:"category_name"`
	FullAddress     string  `json:"full_address"`
	Province        string  `json:"province"`
	City            string  `json:"city"`
	District        string  `json:"district"`
	AdCode          string  `json:"ad_code"`
	Longitude       float64 `json:"longitude"`
	Latitude        float64 `json:"latitude"`
}

type imageSource struct {
	Slug         string `json:"slug"`
	ImageFile    string `json:"image_file"`
	OutputWidth  int    `json:"output_width"`
	OutputHeight int    `json:"output_height"`
}

type presetRecord struct {
	hotPlace hotPlace
	place    placeCatalogItem
	image    []byte
}

func main() {
	records, err := loadPreset()
	if err != nil {
		log.Fatalf("load preset: %v", err)
	}

	for _, record := range records {
		if err := environment.DB.ResourceHandle().SaveResource(
			handle.ResourceTypePlaceImage,
			record.hotPlace.PlaceImageItemID,
			record.image,
		); err != nil {
			log.Fatalf("save image for %s: %v", record.hotPlace.RecommendTitle, err)
		}
	}

	if err := environment.DB.Database().Transaction(func(tx *gorm.DB) error {
		for _, record := range records {
			if err := upsertPlace(tx, record.place); err != nil {
				return err
			}
			if err := upsertHotPlace(tx, record.hotPlace); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		log.Fatalf("import preset into MySQL: %v", err)
	}

	log.Printf("initialized database and imported %d hot places with %d images", len(records), len(records))
}

func loadPreset() ([]presetRecord, error) {
	var hotPlaces []hotPlace
	var places []placeCatalogItem
	var sources []imageSource
	if err := readJSON("hot_places.json", &hotPlaces); err != nil {
		return nil, err
	}
	if err := readJSON("place_catalog.json", &places); err != nil {
		return nil, err
	}
	if err := readJSON("image_sources.json", &sources); err != nil {
		return nil, err
	}
	if len(hotPlaces) == 0 || len(hotPlaces) != len(places) || len(hotPlaces) != len(sources) {
		return nil, fmt.Errorf("record counts must be non-zero and equal, got hot_places=%d places=%d image_sources=%d", len(hotPlaces), len(places), len(sources))
	}

	seenPlaceIDs := make(map[string]struct{}, len(places))
	seenProviderPlaceIDs := make(map[string]struct{}, len(places))
	for index, place := range places {
		if err := validateUUID("place_catalog place_identity", place.PlaceIdentity); err != nil {
			return nil, fmt.Errorf("place_catalog[%d]: %w", index, err)
		}
		if _, exists := seenPlaceIDs[place.PlaceIdentity]; exists {
			return nil, fmt.Errorf("place_catalog[%d] duplicates place_identity %q", index, place.PlaceIdentity)
		}
		if place.ProviderName != define.PlaceProviderNameDefault || strings.TrimSpace(place.ProviderPlaceID) == "" || len(place.ProviderPlaceID) > 64 {
			return nil, fmt.Errorf("place_catalog[%d] must contain an Amap provider name and 1-64 character provider place ID", index)
		}
		if _, exists := seenProviderPlaceIDs[place.ProviderPlaceID]; exists {
			return nil, fmt.Errorf("place_catalog[%d] duplicates provider_place_id %q", index, place.ProviderPlaceID)
		}
		if strings.TrimSpace(place.Name) == "" || strings.TrimSpace(place.CategoryCode) == "" || strings.TrimSpace(place.CategoryName) == "" {
			return nil, fmt.Errorf("place_catalog[%d] has empty Amap name or category", index)
		}
		if err := validateCuratedCategoryName(place.CategoryName); err != nil {
			return nil, fmt.Errorf("place_catalog[%d] has invalid category_name: %w", index, err)
		}
		seenPlaceIDs[place.PlaceIdentity] = struct{}{}
		seenProviderPlaceIDs[place.ProviderPlaceID] = struct{}{}
	}

	seenHotIDs := make(map[string]struct{}, len(hotPlaces))
	seenHotUniqueIDs := make(map[uint32]struct{}, len(hotPlaces))
	seenImageIDs := make(map[string]struct{}, len(hotPlaces))
	records := make([]presetRecord, 0, len(hotPlaces))
	for index, item := range hotPlaces {
		if item.HotPlaceUniqueID == 0 {
			return nil, fmt.Errorf("hot_places[%d] has an empty HotPlaceUniqueID", index)
		}
		for field, value := range map[string]string{
			"HotPlaceIdentity": item.HotPlaceIdentity,
			"PlaceImageItemID": item.PlaceImageItemID,
			"PlaceIdentity":    item.PlaceIdentity,
		} {
			if err := validateUUID(field, value); err != nil {
				return nil, fmt.Errorf("hot_places[%d]: %w", index, err)
			}
		}
		if _, exists := seenHotIDs[item.HotPlaceIdentity]; exists {
			return nil, fmt.Errorf("hot_places[%d] duplicates HotPlaceIdentity %q", index, item.HotPlaceIdentity)
		}
		if _, exists := seenHotUniqueIDs[item.HotPlaceUniqueID]; exists {
			return nil, fmt.Errorf("hot_places[%d] duplicates HotPlaceUniqueID %d", index, item.HotPlaceUniqueID)
		}
		if _, exists := seenImageIDs[item.PlaceImageItemID]; exists {
			return nil, fmt.Errorf("hot_places[%d] duplicates PlaceImageItemID %q", index, item.PlaceImageItemID)
		}
		if _, exists := seenPlaceIDs[item.PlaceIdentity]; !exists {
			return nil, fmt.Errorf("hot_places[%d] references missing PlaceIdentity %q", index, item.PlaceIdentity)
		}
		if strings.TrimSpace(item.RecommendTitle) == "" || utf8.RuneCountInString(item.RecommendTitle) > 64 {
			return nil, fmt.Errorf("hot_places[%d] has an invalid RecommendTitle", index)
		}
		detailRunes := utf8.RuneCountInString(item.RecommandDetail)
		if detailRunes < 700 || detailRunes > 2048 {
			return nil, fmt.Errorf("hot_places[%d] RecommandDetail has %d characters; expected 700-2048", index, detailRunes)
		}

		imageData, err := loadImage(sources[index])
		if err != nil {
			return nil, fmt.Errorf("hot_places[%d] %s: %w", index, item.RecommendTitle, err)
		}
		if places[index].PlaceIdentity != item.PlaceIdentity {
			return nil, fmt.Errorf("hot_places[%d] and place_catalog[%d] do not describe the same PlaceIdentity", index, index)
		}

		seenHotIDs[item.HotPlaceIdentity] = struct{}{}
		seenHotUniqueIDs[item.HotPlaceUniqueID] = struct{}{}
		seenImageIDs[item.PlaceImageItemID] = struct{}{}
		records = append(records, presetRecord{hotPlace: item, place: places[index], image: imageData})
	}
	return records, nil
}

func upsertPlace(tx *gorm.DB, item placeCatalogItem) error {
	var existing define.PlaceInfo
	result := tx.Where("place_identity = ?", item.PlaceIdentity).First(&existing)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("query place %s: %w", item.PlaceIdentity, result.Error)
	}

	now := time.Now().Unix()
	values := map[string]any{
		"provider_name":     item.ProviderName,
		"provider_place_id": item.ProviderPlaceID,
		"place_name":        item.Name,
		"category_code":     item.CategoryCode,
		"category_name":     item.CategoryName,
		"full_address":      item.FullAddress,
		"in_which_province": item.Province,
		"in_which_city":     item.City,
		"in_which_district": item.District,
		"ad_code":           item.AdCode,
		"longitude":         item.Longitude,
		"latitude":          item.Latitude,
		"coordinate_system": define.PlaceCoordinateSystemDefault,
		"place_status":      define.PlaceStatusActive,
		"sync_unix_time":    now,
	}
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		place := define.PlaceInfo{
			PlaceIdentity:    item.PlaceIdentity,
			ProviderName:     item.ProviderName,
			ProviderPlaceID:  item.ProviderPlaceID,
			PlaceName:        item.Name,
			CategoryCode:     item.CategoryCode,
			CategoryName:     item.CategoryName,
			FullAddress:      item.FullAddress,
			InWhichProvince:  item.Province,
			InWhichCity:      item.City,
			InWhichDistrict:  item.District,
			AdCode:           item.AdCode,
			Longitude:        item.Longitude,
			Latitude:         item.Latitude,
			CoordinateSystem: define.PlaceCoordinateSystemDefault,
			PlaceStatus:      define.PlaceStatusActive,
			SyncUnixTime:     now,
		}
		if err := tx.Create(&place).Error; err != nil {
			return fmt.Errorf("create place %s: %w", item.PlaceIdentity, err)
		}
		return nil
	}
	if existing.ProviderName != "" && existing.ProviderName != "preset" && existing.ProviderName != item.ProviderName {
		return fmt.Errorf("place %s belongs to provider %q/%q, not Amap", item.PlaceIdentity, existing.ProviderName, existing.ProviderPlaceID)
	}
	if err := tx.Model(&existing).Updates(values).Error; err != nil {
		return fmt.Errorf("update place %s: %w", item.PlaceIdentity, err)
	}
	return nil
}

func upsertHotPlace(tx *gorm.DB, item hotPlace) error {
	var existing define.HotPlace
	result := tx.Where("hot_place_identity = ?", item.HotPlaceIdentity).First(&existing)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("query hot place %s: %w", item.HotPlaceIdentity, result.Error)
	}
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		var sameUniqueID define.HotPlace
		idResult := tx.Where("hot_place_unique_id = ?", item.HotPlaceUniqueID).First(&sameUniqueID)
		if idResult.Error == nil {
			return fmt.Errorf("HotPlaceUniqueID %d is already used by %s", item.HotPlaceUniqueID, sameUniqueID.HotPlaceIdentity)
		}
		if idResult.Error != nil && !errors.Is(idResult.Error, gorm.ErrRecordNotFound) {
			return fmt.Errorf("query HotPlaceUniqueID %d: %w", item.HotPlaceUniqueID, idResult.Error)
		}
		place := define.HotPlace{
			HotPlaceUniqueID: item.HotPlaceUniqueID,
			HotPlaceIdentity: item.HotPlaceIdentity,
			RecommendTitle:   item.RecommendTitle,
			RecommandDetail:  item.RecommandDetail,
			PlaceImageItemID: item.PlaceImageItemID,
			PlaceIdentity:    item.PlaceIdentity,
		}
		if err := tx.Create(&place).Error; err != nil {
			return fmt.Errorf("create hot place %s: %w", item.HotPlaceIdentity, err)
		}
		return nil
	}
	if existing.HotPlaceUniqueID != item.HotPlaceUniqueID {
		return fmt.Errorf("hot place %s has unique ID %d, expected %d", item.HotPlaceIdentity, existing.HotPlaceUniqueID, item.HotPlaceUniqueID)
	}
	if err := tx.Model(&existing).Updates(map[string]any{
		"recommend_title":     item.RecommendTitle,
		"recommand_detail":    item.RecommandDetail,
		"place_image_item_id": item.PlaceImageItemID,
		"place_identity":      item.PlaceIdentity,
	}).Error; err != nil {
		return fmt.Errorf("update hot place %s: %w", item.HotPlaceIdentity, err)
	}
	return nil
}

func loadImage(source imageSource) ([]byte, error) {
	if strings.TrimSpace(source.Slug) == "" || strings.TrimSpace(source.ImageFile) == "" {
		return nil, errors.New("image source has an empty slug or image file")
	}
	assetPath := path.Clean(source.ImageFile)
	if path.IsAbs(assetPath) || assetPath == "." || assetPath == ".." || strings.HasPrefix(assetPath, "../") {
		return nil, fmt.Errorf("invalid image file path %q", source.ImageFile)
	}
	data, err := presetAssets.ReadFile(assetPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", assetPath, err)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", assetPath, err)
	}
	if format != "jpeg" || config.Width != imageWidth || config.Height != imageHeight {
		return nil, fmt.Errorf("%s is %s %dx%d; expected jpeg %dx%d", assetPath, format, config.Width, config.Height, imageWidth, imageHeight)
	}
	if source.OutputWidth != imageWidth || source.OutputHeight != imageHeight {
		return nil, fmt.Errorf("source metadata for %s must declare %dx%d output", source.ImageFile, imageWidth, imageHeight)
	}
	return data, nil
}

func readJSON(path string, target any) error {
	data, err := presetAssets.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func validateUUID(field string, value string) error {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil {
		return fmt.Errorf("%s must be a non-empty UUID", field)
	}
	return nil
}

func validateCuratedCategoryName(value string) error {
	if strings.Contains(value, "|") {
		return errors.New("must not contain the provider multi-category separator |")
	}
	labels := strings.Split(value, ";")
	if len(labels) != 3 {
		return fmt.Errorf("must contain exactly 3 semicolon-delimited labels, got %d", len(labels))
	}
	providerLabels := map[string]struct{}{
		"风景名胜": {}, "风景名胜相关": {}, "国家级景点": {}, "省级景点": {},
		"地名地址信息": {}, "自然地名": {}, "热点地名": {},
		"生活服务": {}, "生活服务场所": {}, "科教文化服务": {},
	}
	seen := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		if label == "" || label != strings.TrimSpace(label) {
			return errors.New("labels must be non-empty and must not have surrounding whitespace")
		}
		if utf8.RuneCountInString(label) > 16 {
			return fmt.Errorf("label %q exceeds 16 characters", label)
		}
		if _, exists := seen[label]; exists {
			return fmt.Errorf("label %q is repeated", label)
		}
		if _, isProviderLabel := providerLabels[label]; isProviderLabel {
			return fmt.Errorf("label %q is an Amap provider hierarchy label", label)
		}
		seen[label] = struct{}{}
	}
	return nil
}
