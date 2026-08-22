package service

import (
	"net/http"

	"github.com/Happy2018new/evercare-journey-backend/service/auth"
	"github.com/Happy2018new/evercare-journey-backend/service/profile"
	"github.com/gin-gonic/gin"
)

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

func InitAndMakeRouter() *gin.Engine {
	router := gin.Default()

	router.Use(
		SecurityHeaders(),
		LimitRequestBody(DefaultMaxRequestBodyBytes),
	)
	router.NoRoute(
		func(c *gin.Context) {
			c.AbortWithStatus(http.StatusNotFound)
		},
	)

	registerAuthProcessor(router)
	registerProfileProcessor(router)

	return router
}
