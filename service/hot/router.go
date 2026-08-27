package hot

import "github.com/Happy2018new/evercare-journey-backend/service/general"

type HotPlaceData struct {
	HotPlaceIdentity string `json:"hot_place_identity"`
	RecommendTitle   string `json:"recommend_title"`
	RecommendDetail  string `json:"recommend_detail"`
	PlaceIdentity    string `json:"place_identity"`
}

type HotPlaceRequest struct {
	general.BasicSessionInfo
	RequestCount uint8 `json:"request_count"`
}

type HotPlaceResponse struct {
	general.BasicResponseInfo
	PlaceData []HotPlaceData `json:"place_data"`
}

const (
	HotPlaceImageRequestActionGetChecksum uint8 = iota
	HotPlaceImageRequestActionGetImageData
)

type HotPlaceImageRequest struct {
	general.BasicSessionInfo
	HotPlaceIdentity []string `json:"hot_place_identity"`
	RequestAction    []uint8  `json:"request_action"`
}

type QueryHotPlaceImagesResponse struct {
	general.BasicResponseInfo
	Checksums []string `json:"checksums"`
	ImageData [][]byte `json:"image_data"`
	ImageSet  []bool   `json:"image_set"`
}
