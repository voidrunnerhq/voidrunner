package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("sets basic security headers for HTTP", func(t *testing.T) {
		router := gin.New()
		router.Use(SecurityHeaders())
		router.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "test")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		headers := w.Header()
		assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", headers.Get("X-Frame-Options"))
		assert.Equal(t, "1; mode=block", headers.Get("X-XSS-Protection"))
		assert.Equal(t, "strict-origin-when-cross-origin", headers.Get("Referrer-Policy"))
		assert.Equal(t, "default-src 'self'", headers.Get("Content-Security-Policy"))

		// HSTS header should not be set for HTTP requests
		assert.Empty(t, headers.Get("Strict-Transport-Security"))
	})

	t.Run("sets HSTS header for HTTPS requests", func(t *testing.T) {
		router := gin.New()
		router.Use(SecurityHeaders())
		router.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "test")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		// Simulate HTTPS request by setting TLS
		req.TLS = &tls.ConnectionState{}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		headers := w.Header()
		// Basic security headers should still be set
		assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", headers.Get("X-Frame-Options"))
		assert.Equal(t, "1; mode=block", headers.Get("X-XSS-Protection"))
		assert.Equal(t, "strict-origin-when-cross-origin", headers.Get("Referrer-Policy"))
		assert.Equal(t, "default-src 'self'", headers.Get("Content-Security-Policy"))

		// HSTS header should be set for HTTPS requests
		assert.Equal(t, "max-age=31536000; includeSubDomains", headers.Get("Strict-Transport-Security"))
	})
}
