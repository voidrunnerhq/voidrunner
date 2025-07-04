package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	allowedOrigins := []string{"http://localhost:3000", "https://app.example.com"}
	allowedMethods := []string{"GET", "POST", "PUT", "DELETE"}
	allowedHeaders := []string{"Content-Type", "Authorization", "X-Request-ID"}
	
	router := gin.New()
	router.Use(CORS(allowedOrigins, allowedMethods, allowedHeaders))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "test")
	})

	t.Run("sets CORS headers correctly", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "GET")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
		
		exposeHeaders := w.Header().Get("Access-Control-Expose-Headers")
		if exposeHeaders != "" {
			assert.Contains(t, exposeHeaders, "X-Request-ID")
		}
	})

	t.Run("handles actual request after preflight", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "test", w.Body.String())
	})
}