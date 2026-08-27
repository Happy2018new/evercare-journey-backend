package hot

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/http"
	"strings"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/database/handle"
	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/Happy2018new/evercare-journey-backend/service/auth"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/Happy2018new/evercare-journey-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MaxHotPlaceRequestCount limits the amount of recommendation data returned by
// one request. The database currently has no pagination or limit parameter.
const MaxHotPlaceRequestCount = 50

func HandleHotPlace(c *gin.Context) {
	var request HotPlaceRequest

	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusOK, HotPlaceResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("HandleHotPlace", err, define.LangKeyHotPlaceRequestBodyInvalid),
			),
		})
		return
	}

	status, generalErr := auth.ValidateSession(request.BasicSessionInfo)
	if generalErr != nil {
		c.JSON(http.StatusOK, HotPlaceResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("HandleHotPlace")),
		})
		return
	}
	if status != auth.ValidateSessionStatusValidSession {
		c.JSON(http.StatusOK, HotPlaceResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("HandleHotPlace", fmt.Errorf("Failed to validate current session"), define.LangKeyGeneralInvalidSession),
			),
		})
		return
	}

	if request.RequestCount == 0 || int(request.RequestCount) > MaxHotPlaceRequestCount {
		c.JSON(http.StatusOK, HotPlaceResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(
					"HandleHotPlace",
					fmt.Errorf("Invalid request count %d; expected 1-%d", request.RequestCount, MaxHotPlaceRequestCount),
					define.LangKeyHotPlaceRequestCountInvalid,
				),
			),
		})
		return
	}

	hotPlaces, generalErr := environment.DB.HotHandle().QueryMulActivePlace(environment.DB.Database(), "")
	if generalErr != nil {
		c.JSON(http.StatusOK, HotPlaceResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("HandleHotPlace")),
		})
		return
	}
	if len(hotPlaces) < int(request.RequestCount) {
		c.JSON(http.StatusOK, HotPlaceResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(
					"HandleHotPlace",
					fmt.Errorf("requested %d recommendations but only %d active hot places are available", request.RequestCount, len(hotPlaces)),
					define.LangKeyHotPlaceRequestInsufficient,
				),
			),
		})
		return
	}

	// Shuffle indexes rather than the database result itself, preserving the
	// ownership of the slice returned by the handle.
	indexes := rand.Perm(len(hotPlaces))
	resultCount := int(request.RequestCount)
	if resultCount > len(indexes) {
		resultCount = len(indexes)
	}
	placeData := make([]HotPlaceData, 0, resultCount)
	for _, index := range indexes[:resultCount] {
		hotPlace := hotPlaces[index]
		placeData = append(placeData, HotPlaceData{
			HotPlaceIdentity: hotPlace.HotPlaceIdentity,
			RecommendTitle:   hotPlace.RecommendTitle,
			RecommendDetail:  hotPlace.RecommandDetail,
			PlaceIdentity:    hotPlace.PlaceIdentity,
		})
	}

	c.JSON(http.StatusOK, HotPlaceResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		PlaceData:         placeData,
	})
}

func HandleHotPlaceImage(c *gin.Context) {
	var request HotPlaceImageRequest

	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusOK, QueryHotPlaceImagesResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("HandleHotPlaceImage", err, define.LangKeyHotPlaceRequestBodyInvalid),
			),
		})
		return
	}

	status, generalErr := auth.ValidateSession(request.BasicSessionInfo)
	if generalErr != nil {
		c.JSON(http.StatusOK, QueryHotPlaceImagesResponse{
			BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("HandleHotPlaceImage")),
		})
		return
	}
	if status != auth.ValidateSessionStatusValidSession {
		c.JSON(http.StatusOK, QueryHotPlaceImagesResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError("HandleHotPlaceImage", fmt.Errorf("Failed to validate current session"), define.LangKeyGeneralInvalidSession),
			),
		})
		return
	}

	if len(request.HotPlaceIdentity) == 0 || len(request.HotPlaceIdentity) != len(request.RequestAction) {
		c.JSON(http.StatusOK, QueryHotPlaceImagesResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(
					"HandleHotPlaceImage",
					fmt.Errorf("hot place identity and request action must be non-empty and have equal lengths"),
					define.LangKeyHotPlaceImageRequestInvalid,
				),
			),
		})
		return
	}
	if len(request.HotPlaceIdentity) > MaxHotPlaceRequestCount {
		c.JSON(http.StatusOK, QueryHotPlaceImagesResponse{
			BasicResponseInfo: general.FromGeneralError(
				define.NewGeneralError(
					"HandleHotPlaceImage",
					fmt.Errorf("too many hot place images requested; maximum is %d", MaxHotPlaceRequestCount),
					define.LangKeyHotPlaceImageRequestInvalid,
				),
			),
		})
		return
	}

	for index, identity := range request.HotPlaceIdentity {
		identity = strings.TrimSpace(identity)
		if identity == "" {
			c.JSON(http.StatusOK, QueryHotPlaceImagesResponse{
				BasicResponseInfo: general.FromGeneralError(
					define.NewGeneralError(
						"HandleHotPlaceImage",
						fmt.Errorf("hot place identity at index %d is empty", index),
						define.LangKeyHotPlaceIdentityInvalid,
					),
				),
			})
			return
		}
		parsedIdentity, parseErr := uuid.Parse(identity)
		if parseErr != nil || parsedIdentity == uuid.Nil {
			c.JSON(http.StatusOK, QueryHotPlaceImagesResponse{
				BasicResponseInfo: general.FromGeneralError(
					define.NewGeneralError(
						"HandleHotPlaceImage",
						fmt.Errorf("hot place identity at index %d must be a valid UUID", index),
						define.LangKeyHotPlaceIdentityInvalid,
					),
				),
			})
			return
		}
		request.HotPlaceIdentity[index] = parsedIdentity.String()
		if request.RequestAction[index] != HotPlaceImageRequestActionGetChecksum &&
			request.RequestAction[index] != HotPlaceImageRequestActionGetImageData {
			c.JSON(http.StatusOK, QueryHotPlaceImagesResponse{
				BasicResponseInfo: general.FromGeneralError(
					define.NewGeneralError(
						"HandleHotPlaceImage",
						fmt.Errorf("unsupported request action %d at index %d", request.RequestAction[index], index),
						define.LangKeyHotPlaceImageActionInvalid,
					),
				),
			})
			return
		}
	}

	checksums := make([]string, len(request.HotPlaceIdentity))
	imageData := make([][]byte, len(request.HotPlaceIdentity))
	imageSet := make([]bool, len(request.HotPlaceIdentity))
	for index, identity := range request.HotPlaceIdentity {
		hotPlace, found, generalErr := environment.DB.HotHandle().QueryHotPlace(
			environment.DB.Database(),
			handle.QueryHotPlaceActionSearchByIdentity,
			strings.TrimSpace(identity),
		)
		if generalErr != nil {
			c.JSON(http.StatusOK, QueryHotPlaceImagesResponse{
				BasicResponseInfo: general.FromGeneralError(generalErr.AppendSource("HandleHotPlaceImage")),
			})
			return
		}
		if !found {
			c.JSON(http.StatusOK, QueryHotPlaceImagesResponse{
				BasicResponseInfo: general.FromGeneralError(
					define.NewGeneralError(
						"HandleHotPlaceImage",
						fmt.Errorf("target hot place %s not found", identity),
						define.LangKeyHotPlaceQueryNotFoundErr,
					),
				),
			})
			return
		}
		placeActive, placeErr := environment.DB.HotHandle().IsPlaceActive(environment.DB.Database(), hotPlace.PlaceIdentity)
		if placeErr != nil {
			c.JSON(http.StatusOK, QueryHotPlaceImagesResponse{
				BasicResponseInfo: general.FromGeneralError(placeErr.AppendSource("HandleHotPlaceImage")),
			})
			return
		}
		if !placeActive {
			c.JSON(http.StatusOK, QueryHotPlaceImagesResponse{
				BasicResponseInfo: general.FromGeneralError(
					define.NewGeneralError("HandleHotPlaceImage", fmt.Errorf("referenced place is not active"), define.LangKeyHotPlacePlaceInvalid),
				),
			})
			return
		}

		rawImage, found, resourceErr := environment.DB.ResourceHandle().LoadResourceWithError(handle.ResourceTypePlaceImage, hotPlace.PlaceImageItemID)
		if resourceErr != nil {
			c.JSON(http.StatusOK, QueryHotPlaceImagesResponse{
				BasicResponseInfo: general.FromGeneralError(
					define.NewGeneralError("HandleHotPlaceImage", resourceErr, define.LangKeyHotPlaceImageQueryUnknownErr),
				),
			})
			return
		}
		if !found {
			// Images are optional content. Keep the item aligned with the
			// request and let the client render its placeholder instead of
			// failing an otherwise valid batch because one image is unset.
			continue
		}

		imageSet[index] = true
		rawChecksum := sha512.Sum512(rawImage)
		checksums[index] = hex.EncodeToString(rawChecksum[:])
		if request.RequestAction[index] == HotPlaceImageRequestActionGetImageData {
			var compressErr error
			imageData[index], compressErr = utils.CompressBrotli(rawImage)
			if compressErr != nil {
				c.JSON(http.StatusOK, QueryHotPlaceImagesResponse{
					BasicResponseInfo: general.FromGeneralError(
						define.NewGeneralError("HandleHotPlaceImage", compressErr, define.LangKeyHotPlaceImageQueryUnknownErr),
					),
				})
				return
			}
		}
	}

	c.JSON(http.StatusOK, QueryHotPlaceImagesResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		Checksums:         checksums,
		ImageData:         imageData,
		ImageSet:          imageSet,
	})
}
