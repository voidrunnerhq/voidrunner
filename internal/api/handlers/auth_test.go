package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/auth"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
)

// MockAuthService for testing
type MockAuthService struct {
	mock.Mock
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

func (m *MockAuthService) ValidateAccessToken(ctx context.Context, token string) (*models.User, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func setupAuthTest() (*gin.Engine, *AuthHandler, *MockAuthService) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	log := logger.New("debug", "console")
	mockAuth := &MockAuthService{}
	handler := NewAuthHandler(mockAuth, log.Logger)

	return router, handler, mockAuth
}

func TestNewAuthHandler(t *testing.T) {
	mockAuth := &MockAuthService{}
	log := logger.New("debug", "console")

	handler := NewAuthHandler(mockAuth, log.Logger)

	assert.NotNil(t, handler)
	assert.Equal(t, mockAuth, handler.authService)
	assert.Equal(t, log.Logger, handler.logger)
}

func TestAuthHandler_Register_Success(t *testing.T) {
	router, handler, mockAuth := setupAuthTest()

	// Setup expected response
	userID := uuid.New()
	expectedResponse := &models.AuthResponse{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		User: models.UserResponse{
			ID:    userID,
			Email: "test@example.com",
			Name:  "Test User",
		},
	}

	registerReq := models.RegisterRequest{
		Email:    "test@example.com",
		Password: "TestPass123!",
		Name:     "Test User",
	}

	mockAuth.On("Register", mock.Anything, registerReq).Return(expectedResponse, nil)

	router.POST("/register", handler.Register)

	// Create request
	body, _ := json.Marshal(registerReq)
	req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response models.AuthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, expectedResponse.AccessToken, response.AccessToken)
	assert.Equal(t, expectedResponse.User.Email, response.User.Email)

	mockAuth.AssertExpectations(t)
}

func TestAuthHandler_Register_ValidationError(t *testing.T) {
	router, handler, mockAuth := setupAuthTest()

	router.POST("/register", handler.Register)

	testCases := []struct {
		name           string
		body           map[string]interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name: "missing email",
			body: map[string]interface{}{
				"password": "TestPass123!",
				"name":     "Test User",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid email: email is required",
		},
		{
			name: "missing password",
			body: map[string]interface{}{
				"email": "test@example.com",
				"name":  "Test User",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid password: password must be at least 8 characters long",
		},
		{
			name: "missing name",
			body: map[string]interface{}{
				"email":    "test@example.com",
				"password": "TestPass123!",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error: name is required",
		},
		{
			name: "invalid email",
			body: map[string]interface{}{
				"email":    "invalid-email",
				"password": "TestPass123!",
				"name":     "Test User",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid email: invalid email format",
		},
		{
			name: "weak password",
			body: map[string]interface{}{
				"email":    "test@example.com",
				"password": "weak",
				"name":     "Test User",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid password: password must be at least 8 characters long",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the request object that will be passed to the auth service
			var reqObj models.RegisterRequest
			reqBody, _ := json.Marshal(tc.body)
			require.NoError(t, json.Unmarshal(reqBody, &reqObj))

			// Set up mock expectation for the auth service to return validation error
			var mockError error
			switch tc.name {
			case "missing email", "invalid email":
				mockError = fmt.Errorf("%w: %v", auth.ErrInvalidEmail, "test error")
			case "missing password", "weak password":
				mockError = fmt.Errorf("%w: %v", auth.ErrInvalidPassword, "test error")
			case "missing name":
				mockError = fmt.Errorf("%w: %v", auth.ErrValidationFailed, "test error")
			default:
				mockError = fmt.Errorf("%s", tc.expectedError)
			}
			mockAuth.On("Register", mock.Anything, reqObj).Return(nil, mockError)

			req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)

			// Clean up mock expectations for this test case
			mockAuth.ExpectedCalls = nil
		})
	}

	mockAuth.AssertExpectations(t)
}

func TestAuthHandler_Register_ServiceError(t *testing.T) {
	router, handler, mockAuth := setupAuthTest()

	registerReq := models.RegisterRequest{
		Email:    "test@example.com",
		Password: "TestPass123!",
		Name:     "Test User",
	}

	mockAuth.On("Register", mock.Anything, registerReq).Return(nil, auth.ErrUserAlreadyExists)

	router.POST("/register", handler.Register)

	body, _ := json.Marshal(registerReq)
	req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	mockAuth.AssertExpectations(t)
}

func TestAuthHandler_Login_Success(t *testing.T) {
	router, handler, mockAuth := setupAuthTest()

	userID := uuid.New()
	expectedResponse := &models.AuthResponse{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		User: models.UserResponse{
			ID:    userID,
			Email: "test@example.com",
			Name:  "Test User",
		},
	}

	loginReq := models.LoginRequest{
		Email:    "test@example.com",
		Password: "TestPass123!",
	}

	mockAuth.On("Login", mock.Anything, loginReq).Return(expectedResponse, nil)

	router.POST("/login", handler.Login)

	body, _ := json.Marshal(loginReq)
	req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.AuthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, expectedResponse.AccessToken, response.AccessToken)
	assert.Equal(t, expectedResponse.User.Email, response.User.Email)

	mockAuth.AssertExpectations(t)
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	router, handler, mockAuth := setupAuthTest()

	loginReq := models.LoginRequest{
		Email:    "test@example.com",
		Password: "wrongpassword",
	}

	mockAuth.On("Login", mock.Anything, loginReq).Return(nil, auth.ErrInvalidCredentials)

	router.POST("/login", handler.Login)

	body, _ := json.Marshal(loginReq)
	req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	mockAuth.AssertExpectations(t)
}

func TestAuthHandler_RefreshToken_Success(t *testing.T) {
	router, handler, mockAuth := setupAuthTest()

	userID := uuid.New()
	expectedResponse := &models.AuthResponse{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		User: models.UserResponse{
			ID:    userID,
			Email: "test@example.com",
			Name:  "Test User",
		},
	}

	refreshReq := models.RefreshTokenRequest{
		RefreshToken: "valid-refresh-token",
	}

	mockAuth.On("RefreshToken", mock.Anything, refreshReq).Return(expectedResponse, nil)

	router.POST("/refresh", handler.RefreshToken)

	body, _ := json.Marshal(refreshReq)
	req := httptest.NewRequest("POST", "/refresh", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.AuthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, expectedResponse.AccessToken, response.AccessToken)

	mockAuth.AssertExpectations(t)
}

func TestAuthHandler_RefreshToken_InvalidToken(t *testing.T) {
	router, handler, mockAuth := setupAuthTest()

	refreshReq := models.RefreshTokenRequest{
		RefreshToken: "invalid-refresh-token",
	}

	mockAuth.On("RefreshToken", mock.Anything, refreshReq).Return(nil, auth.ErrInvalidRefreshToken)

	router.POST("/refresh", handler.RefreshToken)

	body, _ := json.Marshal(refreshReq)
	req := httptest.NewRequest("POST", "/refresh", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	mockAuth.AssertExpectations(t)
}

func TestAuthHandler_Me_Success(t *testing.T) {
	router, handler, _ := setupAuthTest()

	userID := uuid.New()
	user := &models.User{
		BaseModel: models.BaseModel{ID: userID},
		Email:     "test@example.com",
		Name:      "Test User",
	}

	router.GET("/me", func(c *gin.Context) {
		// Simulate auth middleware setting user
		c.Set("user", user)
		handler.Me(c)
	})

	req := httptest.NewRequest("GET", "/me", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]models.UserResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	userResp, exists := response["user"]
	assert.True(t, exists)
	assert.Equal(t, userID, userResp.ID)
	assert.Equal(t, "test@example.com", userResp.Email)
}

func TestAuthHandler_Me_NoUser(t *testing.T) {
	router, handler, _ := setupAuthTest()

	router.GET("/me", handler.Me)

	req := httptest.NewRequest("GET", "/me", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_Logout_Success(t *testing.T) {
	router, handler, _ := setupAuthTest()

	router.POST("/logout", handler.Logout)

	req := httptest.NewRequest("POST", "/logout", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	message, exists := response["message"]
	assert.True(t, exists)
	assert.Equal(t, "logged out successfully", message)
}

func TestAuthHandler_InvalidJSON(t *testing.T) {
	router, handler, _ := setupAuthTest()

	router.POST("/register", handler.Register)
	router.POST("/login", handler.Login)
	router.POST("/refresh", handler.RefreshToken)

	endpoints := []string{"/register", "/login", "/refresh"}

	for _, endpoint := range endpoints {
		t.Run("invalid_json_"+endpoint, func(t *testing.T) {
			req := httptest.NewRequest("POST", endpoint, bytes.NewBuffer([]byte("invalid json")))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// Benchmark tests
func BenchmarkAuthHandler_Register(b *testing.B) {
	router, handler, mockAuth := setupAuthTest()

	userID := uuid.New()
	expectedResponse := &models.AuthResponse{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		User: models.UserResponse{
			ID:    userID,
			Email: "test@example.com",
			Name:  "Test User",
		},
	}

	mockAuth.On("Register", mock.Anything, mock.Anything).Return(expectedResponse, nil)

	router.POST("/register", handler.Register)

	registerReq := models.RegisterRequest{
		Email:    "test@example.com",
		Password: "TestPass123!",
		Name:     "Test User",
	}

	body, _ := json.Marshal(registerReq)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
	}
}
