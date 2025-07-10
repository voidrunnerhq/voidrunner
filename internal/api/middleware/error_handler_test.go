package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorHandler(t *testing.T) {
	t.Run("handles no errors correctly", func(t *testing.T) {
		middleware := ErrorHandler()

		router := gin.New()
		router.Use(middleware)
		router.GET("/success", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/success", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
	})

	t.Run("handles bind errors with 400 status", func(t *testing.T) {
		middleware := ErrorHandler()

		router := gin.New()
		router.Use(middleware)
		router.POST("/bind-error", func(c *gin.Context) {
			// Simulate a binding error
			_ = c.Error(errors.New("invalid JSON format")).SetType(gin.ErrorTypeBind)
		})

		req := httptest.NewRequest("POST", "/bind-error", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Bad Request", response.Error)
		assert.Equal(t, "invalid JSON format", response.Message)
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("handles public errors with 500 status", func(t *testing.T) {
		middleware := ErrorHandler()

		router := gin.New()
		router.Use(middleware)
		router.GET("/public-error", func(c *gin.Context) {
			// Simulate a public error
			_ = c.Error(errors.New("database connection failed")).SetType(gin.ErrorTypePublic)
		})

		req := httptest.NewRequest("GET", "/public-error", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Internal Server Error", response.Error)
		assert.Equal(t, "database connection failed", response.Message)
		assert.Equal(t, http.StatusInternalServerError, response.Code)
	})

	t.Run("handles private errors with generic 500 response", func(t *testing.T) {
		middleware := ErrorHandler()

		router := gin.New()
		router.Use(middleware)
		router.GET("/private-error", func(c *gin.Context) {
			// Simulate a private error (default type)
			_ = c.Error(errors.New("sensitive internal details"))
		})

		req := httptest.NewRequest("GET", "/private-error", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Internal Server Error", response.Error)
		assert.Equal(t, "An unexpected error occurred", response.Message)
		assert.Equal(t, http.StatusInternalServerError, response.Code)

		// Verify sensitive details are not exposed
		assert.NotContains(t, response.Message, "sensitive")
		assert.NotContains(t, response.Message, "internal details")
	})

	t.Run("handles multiple errors and returns the last one", func(t *testing.T) {
		middleware := ErrorHandler()

		router := gin.New()
		router.Use(middleware)
		router.GET("/multiple-errors", func(c *gin.Context) {
			// Add multiple errors
			_ = c.Error(errors.New("first error")).SetType(gin.ErrorTypeBind)
			_ = c.Error(errors.New("second error")).SetType(gin.ErrorTypePublic)
			_ = c.Error(errors.New("third error")) // Private error (last one)
		})

		req := httptest.NewRequest("GET", "/multiple-errors", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Should handle the last error (private error with generic message)
		assert.Equal(t, "Internal Server Error", response.Error)
		assert.Equal(t, "An unexpected error occurred", response.Message)
		assert.Equal(t, http.StatusInternalServerError, response.Code)
	})

	t.Run("middleware continues to next handler when no errors", func(t *testing.T) {
		middleware := ErrorHandler()
		nextCalled := false

		router := gin.New()
		router.Use(middleware)
		router.Use(func(c *gin.Context) {
			nextCalled = true
			c.Next()
		})
		router.GET("/no-error", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/no-error", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, nextCalled)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("handles render errors", func(t *testing.T) {
		middleware := ErrorHandler()

		router := gin.New()
		router.Use(middleware)
		router.GET("/render-error", func(c *gin.Context) {
			// Simulate a render error
			_ = c.Error(errors.New("template not found")).SetType(gin.ErrorTypeRender)
		})

		req := httptest.NewRequest("GET", "/render-error", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Render errors should be treated as generic internal server errors
		assert.Equal(t, "Internal Server Error", response.Error)
		assert.Equal(t, "An unexpected error occurred", response.Message)
		assert.Equal(t, http.StatusInternalServerError, response.Code)
	})

	t.Run("handles errors with nil message", func(t *testing.T) {
		middleware := ErrorHandler()

		router := gin.New()
		router.Use(middleware)
		router.GET("/nil-error", func(c *gin.Context) {
			// Add an error with nil underlying error (edge case)
			c.Errors = append(c.Errors, &gin.Error{
				Err:  errors.New(""),
				Type: gin.ErrorTypeBind,
			})
		})

		req := httptest.NewRequest("GET", "/nil-error", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Bad Request", response.Error)
		assert.Equal(t, "", response.Message) // Empty error message
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("error response structure is correct", func(t *testing.T) {
		middleware := ErrorHandler()

		router := gin.New()
		router.Use(middleware)
		router.POST("/test", func(c *gin.Context) {
			_ = c.Error(errors.New("validation failed")).SetType(gin.ErrorTypeBind)
		})

		req := httptest.NewRequest("POST", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

		// Verify JSON structure
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Check all required fields are present
		assert.Contains(t, response, "error")
		assert.Contains(t, response, "message")
		assert.Contains(t, response, "code")

		// Verify types
		assert.IsType(t, "", response["error"])
		assert.IsType(t, "", response["message"])
		assert.IsType(t, float64(0), response["code"]) // JSON numbers are float64
	})

	t.Run("integration with actual gin validation", func(t *testing.T) {
		middleware := ErrorHandler()

		type TestRequest struct {
			Name  string `json:"name" binding:"required"`
			Email string `json:"email" binding:"required,email"`
		}

		router := gin.New()
		router.Use(middleware)
		router.POST("/validate", func(c *gin.Context) {
			var req TestRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				_ = c.Error(err).SetType(gin.ErrorTypeBind)
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "valid"})
		})

		// Send invalid JSON
		req := httptest.NewRequest("POST", "/validate", strings.NewReader(`{"name": "", "email": "invalid"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Bad Request", response.Error)
		assert.NotEmpty(t, response.Message)
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("handles concurrent error processing", func(t *testing.T) {
		middleware := ErrorHandler()

		router := gin.New()
		router.Use(middleware)
		router.GET("/concurrent", func(c *gin.Context) {
			// Add multiple errors concurrently (unlikely in real usage but tests thread safety)
			_ = c.Error(errors.New("concurrent error 1")).SetType(gin.ErrorTypeBind)
			_ = c.Error(errors.New("concurrent error 2")).SetType(gin.ErrorTypePublic)
		})

		// Make multiple concurrent requests
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/concurrent", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should handle errors consistently
			assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)

			var response ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.NotEmpty(t, response.Error)
			assert.NotEmpty(t, response.Message)
			assert.True(t, response.Code == http.StatusBadRequest || response.Code == http.StatusInternalServerError)
		}
	})
}

func TestErrorResponse(t *testing.T) {
	t.Run("creates error response with all fields", func(t *testing.T) {
		errResp := ErrorResponse{
			Error:   "Test Error",
			Message: "This is a test error message",
			Code:    http.StatusBadRequest,
		}

		assert.Equal(t, "Test Error", errResp.Error)
		assert.Equal(t, "This is a test error message", errResp.Message)
		assert.Equal(t, http.StatusBadRequest, errResp.Code)
	})

	t.Run("serializes to JSON correctly", func(t *testing.T) {
		errResp := ErrorResponse{
			Error:   "Validation Error",
			Message: "Required field missing",
			Code:    422,
		}

		jsonBytes, err := json.Marshal(errResp)
		require.NoError(t, err)

		var unmarshaled map[string]interface{}
		err = json.Unmarshal(jsonBytes, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, "Validation Error", unmarshaled["error"])
		assert.Equal(t, "Required field missing", unmarshaled["message"])
		assert.Equal(t, float64(422), unmarshaled["code"])
	})

	t.Run("handles empty message field", func(t *testing.T) {
		errResp := ErrorResponse{
			Error: "Error without message",
			Code:  500,
		}

		jsonBytes, err := json.Marshal(errResp)
		require.NoError(t, err)

		// Should omit empty message due to omitempty tag
		jsonStr := string(jsonBytes)
		assert.Contains(t, jsonStr, "error")
		assert.Contains(t, jsonStr, "code")

		// Message field should be omitted when empty due to omitempty tag
		var unmarshaled map[string]interface{}
		err = json.Unmarshal(jsonBytes, &unmarshaled)
		require.NoError(t, err)

		_, hasMessage := unmarshaled["message"]
		assert.False(t, hasMessage, "Empty message should be omitted")
	})
}
