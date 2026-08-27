// The preset command initializes the application's tables and imports the
// bundled nationwide hot-place pack. Run it from the repository root with:
//
//	go run ./preset
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"log"
	"os"
	"path/filepath"
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
	presetDirectory    = "preset"
	presetProviderName = "preset"
	imageWidth         = 1920
	imageHeight        = 1080
)

type hotPlace struct {
	HotPlaceUniqueID uint32 `json:"HotPlaceUniqueID"`
	HotPlaceIdentity string `json:"HotPlaceIdentity"`
	RecommendTitle   string `json:"RecommendTitle"`
	RecommandDetail  string `json:"RecommandDetail"`
	PlaceImageItemID string `json:"PlaceImageItemID"`
	PlaceIdentity    string `json:"PlaceIdentity"`
}

type placeCatalogItem struct {
	PlaceIdentity string  `json:"place_identity"`
	Name          string  `json:"name"`
	Province      string  `json:"province"`
	City          string  `json:"city"`
	Longitude     float64 `json:"longitude"`
	Latitude      float64 `json:"latitude"`
	Category      string  `json:"category"`
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
	records, err := loadPreset(presetDirectory)
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

func loadPreset(directory string) ([]presetRecord, error) {
	var hotPlaces []hotPlace
	var places []placeCatalogItem
	var sources []imageSource
	if err := readJSON(filepath.Join(directory, "hot_places.json"), &hotPlaces); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(directory, "place_catalog.json"), &places); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(directory, "image_sources.json"), &sources); err != nil {
		return nil, err
	}
	if len(hotPlaces) == 0 || len(hotPlaces) != len(places) || len(hotPlaces) != len(sources) {
		return nil, fmt.Errorf("record counts must be non-zero and equal, got hot_places=%d places=%d image_sources=%d", len(hotPlaces), len(places), len(sources))
	}

	seenPlaceIDs := make(map[string]struct{}, len(places))
	for index, place := range places {
		if err := validateUUID("place_catalog place_identity", place.PlaceIdentity); err != nil {
			return nil, fmt.Errorf("place_catalog[%d]: %w", index, err)
		}
		if _, exists := seenPlaceIDs[place.PlaceIdentity]; exists {
			return nil, fmt.Errorf("place_catalog[%d] duplicates place_identity %q", index, place.PlaceIdentity)
		}
		if strings.TrimSpace(place.Name) == "" || strings.TrimSpace(place.Category) == "" {
			return nil, fmt.Errorf("place_catalog[%d] has empty name or category", index)
		}
		seenPlaceIDs[place.PlaceIdentity] = struct{}{}
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

		imageData, err := loadImage(directory, sources[index])
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
		"place_name":        item.Name,
		"category_code":     "preset",
		"category_name":     item.Category,
		"full_address":      strings.TrimSpace(item.Province + item.City + item.Name),
		"in_which_province": item.Province,
		"in_which_city":     item.City,
		"in_which_district": "",
		"ad_code":           "",
		"longitude":         item.Longitude,
		"latitude":          item.Latitude,
		"coordinate_system": define.PlaceCoordinateSystemDefault,
		"place_status":      define.PlaceStatusActive,
		"sync_unix_time":    now,
	}
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		place := define.PlaceInfo{
			PlaceIdentity:    item.PlaceIdentity,
			ProviderName:     presetProviderName,
			ProviderPlaceID:  item.PlaceIdentity,
			PlaceName:        item.Name,
			CategoryCode:     "preset",
			CategoryName:     item.Category,
			FullAddress:      strings.TrimSpace(item.Province + item.City + item.Name),
			InWhichProvince:  item.Province,
			InWhichCity:      item.City,
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
	if existing.ProviderName != presetProviderName || existing.ProviderPlaceID != item.PlaceIdentity {
		return fmt.Errorf("place %s belongs to provider %q/%q, not this preset", item.PlaceIdentity, existing.ProviderName, existing.ProviderPlaceID)
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

func loadImage(directory string, source imageSource) ([]byte, error) {
	if strings.TrimSpace(source.Slug) == "" || strings.TrimSpace(source.ImageFile) == "" {
		return nil, errors.New("image source has an empty slug or image file")
	}
	relativePath := filepath.Clean(filepath.FromSlash(source.ImageFile))
	if filepath.IsAbs(relativePath) || relativePath == "." || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("invalid image file path %q", source.ImageFile)
	}
	imagePath := filepath.Join(directory, relativePath)
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", imagePath, err)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", imagePath, err)
	}
	if format != "jpeg" || config.Width != imageWidth || config.Height != imageHeight {
		return nil, fmt.Errorf("%s is %s %dx%d; expected jpeg %dx%d", imagePath, format, config.Width, config.Height, imageWidth, imageHeight)
	}
	if source.OutputWidth != imageWidth || source.OutputHeight != imageHeight {
		return nil, fmt.Errorf("source metadata for %s must declare %dx%d output", source.ImageFile, imageWidth, imageHeight)
	}
	return data, nil
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
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
