package service

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AllowCors handles the browser preflight required by the H5 client when it
// posts JSON to the API from a different origin.
func AllowCors() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "POST, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// DefaultMaxRequestBodyBytes is the maximum accepted HTTP request body size.
const DefaultMaxRequestBodyBytes int64 = 8 * 1024 * 1024

// SecurityHeaders adds response headers suitable for a JSON-only API.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		c.Header("Permissions-Policy", "camera=(), geolocation=(), microphone=()")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Next()
	}
}

// LimitRequestBody rejects bodies larger than maxBytes, including chunked requests
// that do not include a Content-Length header.
func LimitRequestBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes < 0 || c.Request.ContentLength > maxBytes {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}

		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBytes+1))
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if int64(len(body)) > maxBytes {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}

		c.Request.Body.Close()
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Request.ContentLength = int64(len(body))
		c.Next()
	}
}
