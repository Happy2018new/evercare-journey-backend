package service

import (
	_ "embed"
	"net/http"

	"github.com/Happy2018new/evercare-journey-backend/service/auth"
	"github.com/Happy2018new/evercare-journey-backend/service/family"
	"github.com/Happy2018new/evercare-journey-backend/service/hot"
	"github.com/Happy2018new/evercare-journey-backend/service/profile"
	"github.com/Happy2018new/evercare-journey-backend/service/trip"
	"github.com/gin-gonic/gin"
)

//go:embed setup.txt
var setupContent string

func registerStepupProcessor(router *gin.Engine) {
	router.GET(
		"/",
		func(c *gin.Context) {
			c.String(http.StatusOK, setupContent)
		},
	)
}

func registerAuthProcessor(router *gin.Engine) {
	authGroup := router.Group("/auth")
	authGroup.POST("/login", auth.HandleLogin)
	authGroup.POST("/check", auth.HandleSessionCheck)
}

func registerProfileProcessor(router *gin.Engine) {
	profileGroup := router.Group("/profile")
	profileGroup.POST("/data_request", profile.HandleProfileData)
	profileGroup.POST("/avatar_query", profile.HandleAvatarQuery)
	profileGroup.POST("/avatar_upload", profile.HandleAvatarUpload)
}

func registerTripProcessor(router *gin.Engine) {
	tripGroup := router.Group("/trip")
	tripGroup.POST("/place_by_identity", trip.HandlePlaceByIdentity)
	tripGroup.POST("/query_place", trip.HandleQueryPlace)
	tripGroup.POST("/nearby_place", trip.HandleNearbyPlace)
	tripGroup.POST("/create", trip.HandleCreateTrip)
	tripGroup.POST("/query", trip.HandleQueryTrips)
	tripGroup.POST("/query_owned", trip.HandleQueryOwnedTrips)
	tripGroup.POST("/query_version", trip.HandleQueryTripVersion)
	tripGroup.POST("/update", trip.HandleUpdateTrip)
	tripGroup.POST("/optimize", trip.HandleOptimizeTrip)
	tripGroup.POST("/edit_node", trip.HandleEditTripNode)
	tripGroup.POST("/delete", trip.HandleDeleteTrip)
}

func registerHotProcessor(router *gin.Engine) {
	hotGroup := router.Group("/hot")
	hotGroup.POST("/place", hot.HandleHotPlace)
	hotGroup.POST("/place_image", hot.HandleHotPlaceImage)
}

func registerFamilyProcessor(router *gin.Engine) {
	familyGroup := router.Group("/family")
	familyGroup.POST("/create", family.HandleCreate)
	familyGroup.POST("/query", family.HandleQuery)
	familyGroup.POST("/name", family.HandleUpdateName)
	familyGroup.POST("/invite_code", family.HandleGenerateCode)
	familyGroup.POST("/join", family.HandleJoin)
	familyGroup.POST("/leave", family.HandleLeave)
	familyGroup.POST("/member/permission", family.HandleUpdateMemberPermission)
	familyGroup.POST("/member/remove", family.HandleRemoveMember)
	familyGroup.POST("/trip/pin", family.HandlePinTrip)
	familyGroup.POST("/trip/unpin", family.HandleUnpinTrip)
}

func InitAndMakeRouter() *gin.Engine {
	router := gin.Default()
	router.SetTrustedProxies(nil)

	router.Use(
		AllowCors(),
		SecurityHeaders(),
		LimitRequestBody(DefaultMaxRequestBodyBytes),
	)
	router.NoRoute(
		func(c *gin.Context) {
			c.AbortWithStatus(http.StatusNotFound)
		},
	)

	registerStepupProcessor(router)
	registerAuthProcessor(router)
	registerProfileProcessor(router)
	registerTripProcessor(router)
	registerHotProcessor(router)
	registerFamilyProcessor(router)

	return router
}
