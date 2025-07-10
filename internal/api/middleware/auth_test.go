package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
)

// MockAuthService mocks the auth service
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) ValidateAccessToken(ctx context.Context, token string) (*models.User, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockAuthService) Register(ctx context.Context, req models.RegisterRequest) (*models.AuthResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AuthResponse), args.Error(1)
}

func (m *MockAuthService) Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AuthResponse), args.Error(1)
}

func (m *MockAuthService) RefreshToken(ctx context.Context, req models.RefreshTokenRequest) (*models.AuthResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AuthResponse), args.Error(1)
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestNewAuthMiddleware(t *testing.T) {
	mockAuth := &MockAuthService{}
	log := logger.New("debug", "console")

	middleware := NewAuthMiddleware(mockAuth, log.Logger)

	assert.NotNil(t, middleware)
	assert.Equal(t, mockAuth, middleware.authService)
	assert.Equal(t, log.Logger, middleware.logger)
}

func TestAuthMiddleware_RequireAuth_Success(t *testing.T) {
	mockAuth := &MockAuthService{}
	log := logger.New("debug", "console")
	middleware := NewAuthMiddleware(mockAuth, log.Logger)

	userID := uuid.New()
	user := &models.User{
		BaseModel: models.BaseModel{ID: userID},
		Email:     "test@example.com",
		Name:      "Test User",
	}

	mockAuth.On("ValidateAccessToken", mock.Anything, "valid-token").Return(user, nil)

	router := setupTestRouter()
	router.Use(middleware.RequireAuth())
	router.GET("/protected", func(c *gin.Context) {
		user := GetUserFromContext(c)
		assert.NotNil(t, user)
		assert.Equal(t, userID, user.ID)
		assert.Equal(t, "test@example.com", user.Email)
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockAuth.AssertExpectations(t)
}

func TestAuthMiddleware_RequireAuth_MissingToken(t *testing.T) {
	mockAuth := &MockAuthService{}
	log := logger.New("debug", "console")
	middleware := NewAuthMiddleware(mockAuth, log.Logger)

	router := setupTestRouter()
	router.Use(middleware.RequireAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	mockAuth.AssertExpectations(t)
}

func TestAuthMiddleware_RequireAuth_InvalidToken(t *testing.T) {
	mockAuth := &MockAuthService{}
	log := logger.New("debug", "console")
	middleware := NewAuthMiddleware(mockAuth, log.Logger)

	mockAuth.On("ValidateAccessToken", mock.Anything, "invalid-token").Return(nil, fmt.Errorf("invalid token"))

	router := setupTestRouter()
	router.Use(middleware.RequireAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	mockAuth.AssertExpectations(t)
}

func TestAuthMiddleware_RequireAuth_MalformedHeader(t *testing.T) {
	mockAuth := &MockAuthService{}
	log := logger.New("debug", "console")
	middleware := NewAuthMiddleware(mockAuth, log.Logger)

	router := setupTestRouter()
	router.Use(middleware.RequireAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	testCases := []struct {
		name   string
		header string
	}{
		{"no bearer prefix", "just-a-token"},
		{"empty bearer", "Bearer "},
		{"wrong prefix", "Basic token"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", tc.header)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}

	mockAuth.AssertExpectations(t)
}

func TestAuthMiddleware_OptionalAuth_WithToken(t *testing.T) {
	mockAuth := &MockAuthService{}
	log := logger.New("debug", "console")
	middleware := NewAuthMiddleware(mockAuth, log.Logger)

	userID := uuid.New()
	user := &models.User{
		BaseModel: models.BaseModel{ID: userID},
		Email:     "test@example.com",
		Name:      "Test User",
	}

	mockAuth.On("ValidateAccessToken", mock.Anything, "valid-token").Return(user, nil)

	router := setupTestRouter()
	router.Use(middleware.OptionalAuth())
	router.GET("/optional", func(c *gin.Context) {
		user := GetUserFromContext(c)
		if user != nil {
			c.JSON(http.StatusOK, gin.H{"user": user.Email})
		} else {
			c.JSON(http.StatusOK, gin.H{"user": "anonymous"})
		}
	})

	req := httptest.NewRequest("GET", "/optional", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test@example.com")
	mockAuth.AssertExpectations(t)
}

func TestAuthMiddleware_OptionalAuth_WithoutToken(t *testing.T) {
	mockAuth := &MockAuthService{}
	log := logger.New("debug", "console")
	middleware := NewAuthMiddleware(mockAuth, log.Logger)

	router := setupTestRouter()
	router.Use(middleware.OptionalAuth())
	router.GET("/optional", func(c *gin.Context) {
		user := GetUserFromContext(c)
		if user != nil {
			c.JSON(http.StatusOK, gin.H{"user": user.Email})
		} else {
			c.JSON(http.StatusOK, gin.H{"user": "anonymous"})
		}
	})

	req := httptest.NewRequest("GET", "/optional", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "anonymous")
	mockAuth.AssertExpectations(t)
}

func TestAuthMiddleware_OptionalAuth_InvalidToken(t *testing.T) {
	mockAuth := &MockAuthService{}
	log := logger.New("debug", "console")
	middleware := NewAuthMiddleware(mockAuth, log.Logger)

	mockAuth.On("ValidateAccessToken", mock.Anything, "invalid-token").Return(nil, fmt.Errorf("invalid token"))

	router := setupTestRouter()
	router.Use(middleware.OptionalAuth())
	router.GET("/optional", func(c *gin.Context) {
		user := GetUserFromContext(c)
		if user != nil {
			c.JSON(http.StatusOK, gin.H{"user": user.Email})
		} else {
			c.JSON(http.StatusOK, gin.H{"user": "anonymous"})
		}
	})

	req := httptest.NewRequest("GET", "/optional", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "anonymous")
	mockAuth.AssertExpectations(t)
}

func TestExtractToken(t *testing.T) {
	testCases := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "valid bearer token",
			header:   "Bearer valid-token-123",
			expected: "valid-token-123",
		},
		{
			name:     "empty header",
			header:   "",
			expected: "",
		},
		{
			name:     "no bearer prefix",
			header:   "just-a-token",
			expected: "",
		},
		{
			name:     "bearer with no token",
			header:   "Bearer ",
			expected: "",
		},
		{
			name:     "bearer with only spaces",
			header:   "Bearer   ",
			expected: "",
		},
		{
			name:     "wrong case",
			header:   "bearer token-123",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest("GET", "/test", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			c.Request = req

			mockAuth := &MockAuthService{}
			log := logger.New("debug", "console")
			middleware := NewAuthMiddleware(mockAuth, log.Logger)

			token := middleware.extractToken(c)
			assert.Equal(t, tc.expected, token)
		})
	}
}

func TestGetUserFromContext(t *testing.T) {
	// Test with user in context
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	userID := uuid.New()
	user := &models.User{
		BaseModel: models.BaseModel{ID: userID},
		Email:     "test@example.com",
		Name:      "Test User",
	}
	c.Set("user", user)

	retrievedUser := GetUserFromContext(c)
	assert.NotNil(t, retrievedUser)
	assert.Equal(t, userID, retrievedUser.ID)
	assert.Equal(t, "test@example.com", retrievedUser.Email)

	// Test without user in context
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	user2 := GetUserFromContext(c2)
	assert.Nil(t, user2)

	// Test with wrong type in context
	c3, _ := gin.CreateTestContext(httptest.NewRecorder())
	c3.Set("user", "not-a-claims-object")
	user3 := GetUserFromContext(c3)
	assert.Nil(t, user3)
}

func TestRequireUserID(t *testing.T) {
	userID := uuid.New()
	user := &models.User{
		BaseModel: models.BaseModel{ID: userID},
		Email:     "test@example.com",
		Name:      "Test User",
	}

	mockAuth := &MockAuthService{}
	log := logger.New("debug", "console")
	middleware := NewAuthMiddleware(mockAuth, log.Logger)

	// Test with matching user ID - this requires middleware functionality
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.Use(middleware.RequireUserID())
	router.GET("/users/:user_id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/users/"+userID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test with non-matching user ID
	req2 := httptest.NewRequest("GET", "/users/"+uuid.New().String(), nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusForbidden, w2.Code)

	mockAuth.AssertExpectations(t)
}

func TestRequireUserID_MissingUserContext(t *testing.T) {
	logger := logger.New("test", "error")
	middleware := NewAuthMiddleware(nil, logger.Logger)

	router := gin.New()
	router.Use(middleware.RequireUserID())
	router.GET("/users/:user_id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/users/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Unauthorized")
}

func TestRequireUserID_MissingUserIDParameter(t *testing.T) {
	logger := logger.New("test", "error")
	middleware := NewAuthMiddleware(nil, logger.Logger)

	userID := uuid.New()
	user := &models.User{
		BaseModel: models.BaseModel{ID: userID},
		Email:     "test@example.com",
		Name:      "Test User",
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.Use(middleware.RequireUserID())
	// No :user_id parameter in route
	router.GET("/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "User ID parameter required")
}

func TestRequireUserID_InvalidUserType(t *testing.T) {
	logger := logger.New("test", "error")
	middleware := NewAuthMiddleware(nil, logger.Logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		// Set invalid user type (string instead of *models.User)
		c.Set("user", "invalid-user-type")
		c.Next()
	})
	router.Use(middleware.RequireUserID())
	router.GET("/users/:user_id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/users/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Internal server error")
}

func TestRequireUserID_InvalidUUIDFormat(t *testing.T) {
	logger := logger.New("test", "error")
	middleware := NewAuthMiddleware(nil, logger.Logger)

	userID := uuid.New()
	user := &models.User{
		BaseModel: models.BaseModel{ID: userID},
		Email:     "test@example.com",
		Name:      "Test User",
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.Use(middleware.RequireUserID())
	router.GET("/users/:user_id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Use invalid UUID format
	req := httptest.NewRequest("GET", "/users/invalid-uuid-format", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Access denied")
}

func TestRequireUserID_CompleteWorkflow(t *testing.T) {
	logger := logger.New("test", "error")
	mockAuth := new(MockAuthService)
	middleware := NewAuthMiddleware(mockAuth, logger.Logger)

	userID := uuid.New()
	user := &models.User{
		BaseModel: models.BaseModel{ID: userID},
		Email:     "test@example.com",
		Name:      "Test User",
	}

	// Setup mock for authentication
	mockAuth.On("ValidateAccessToken", mock.Anything, "valid-token").Return(user, nil)

	router := gin.New()
	router.Use(middleware.RequireAuth())
	router.Use(middleware.RequireUserID())
	router.GET("/users/:user_id/profile", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "profile data", "user_id": c.Param("user_id")})
	})

	// Test with valid token and matching user ID
	req := httptest.NewRequest("GET", "/users/"+userID.String()+"/profile", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "profile data")

	// Test with valid token but different user ID
	differentUserID := uuid.New()
	req2 := httptest.NewRequest("GET", "/users/"+differentUserID.String()+"/profile", nil)
	req2.Header.Set("Authorization", "Bearer valid-token")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusForbidden, w2.Code)
	assert.Contains(t, w2.Body.String(), "Access denied")

	mockAuth.AssertExpectations(t)
}

// Integration test with real JWT service
func TestAuthMiddleware_Integration(t *testing.T) {
	// This test requires the JWT service to be working correctly
	// It's more of an integration test but validates the middleware works with real tokens
	t.Skip("Integration test - requires full JWT service setup")
}
