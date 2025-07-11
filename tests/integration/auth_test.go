//go:build integration

package integration_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/voidrunnerhq/voidrunner/internal/api/routes"
	"github.com/voidrunnerhq/voidrunner/internal/auth"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
	"github.com/voidrunnerhq/voidrunner/tests/testutil"
)

// AuthIntegrationSuite provides comprehensive authentication integration testing with real JWT validation
type AuthIntegrationSuite struct {
	suite.Suite
	DB          *testutil.DatabaseHelper
	HTTP        *testutil.HTTPHelper
	Auth        *testutil.AuthHelper
	Factory     *testutil.RequestFactory
	Config      *config.Config
	AuthService *auth.Service
	JWTService  *auth.JWTService
}

// SetupSuite initializes the authentication integration test suite
func (s *AuthIntegrationSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)

	// Initialize test configuration with shorter token durations for testing
	s.Config = testutil.GetTestConfig()
	s.Config.JWT.AccessTokenDuration = 30 * time.Second // Short duration for testing expiry
	s.Config.JWT.RefreshTokenDuration = 5 * time.Minute // Longer but still testable

	// Initialize logger
	log := logger.New("test", "debug")

	// Initialize database helper
	s.DB = testutil.NewDatabaseHelper(s.T())

	// Initialize JWT service and auth service
	s.JWTService = auth.NewJWTService(&s.Config.JWT)
	s.AuthService = auth.NewService(s.DB.Repositories.Users, s.JWTService, log.Logger, s.Config)

	// Setup router with full middleware stack
	router := gin.New()
	routes.Setup(router, s.Config, log, s.DB.DB, s.DB.Repositories, s.AuthService)

	// Initialize helpers
	s.HTTP = testutil.NewHTTPHelper(router, s.AuthService)
	s.Auth = testutil.NewAuthHelper(s.AuthService)
	s.Factory = testutil.NewRequestFactory()
}

// TearDownSuite cleans up the authentication integration test suite
func (s *AuthIntegrationSuite) TearDownSuite() {
	if s.DB != nil {
		s.DB.Close()
	}
}

// SetupTest runs before each test
func (s *AuthIntegrationSuite) SetupTest() {
	s.DB.CleanupDatabase(s.T())
}

// TestAuthIntegrationSuite runs the authentication integration test suite
func TestAuthIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping authentication integration tests in short mode")
	}

	suite.Run(t, new(AuthIntegrationSuite))
}

// TestUserRegistrationFlow tests the complete user registration workflow
func (s *AuthIntegrationSuite) TestUserRegistrationFlow() {
	s.Run("successful user registration with valid data", func() {
		registerReq := s.Factory.ValidRegisterRequestWithEmail("register@example.com")
		resp := s.HTTP.POST(s.T(), "/api/v1/auth/register", registerReq).ExpectCreated().ExpectJSON()

		var authResponse models.AuthResponse
		resp.UnmarshalResponse(&authResponse)

		// Validate response structure
		testutil.ValidateAuthTokens(s.T(), &authResponse)
		assert.Equal(s.T(), registerReq.Email, authResponse.User.Email)
		assert.Equal(s.T(), registerReq.Name, authResponse.User.Name)
		assert.NotEmpty(s.T(), authResponse.User.ID)

		// Verify JWT token structure and validity
		s.validateJWTToken(authResponse.AccessToken, authResponse.User.ID.String())
		s.validateJWTToken(authResponse.RefreshToken, authResponse.User.ID.String())

		// Verify user can access protected endpoints with token
		headers := s.Factory.GetAuthHeaders(authResponse.AccessToken)
		meReq := testutil.NewRequest("GET", "/api/v1/auth/me")
		for key, value := range headers {
			meReq.WithHeader(key, value)
		}

		meResp := s.HTTP.Do(s.T(), meReq).ExpectOK().ExpectJSON()

		var meResponseWrapper map[string]models.UserResponse
		meResp.UnmarshalResponse(&meResponseWrapper)

		userResponse := meResponseWrapper["user"]

		assert.Equal(s.T(), authResponse.User.ID, userResponse.ID)
		assert.Equal(s.T(), authResponse.User.Email, userResponse.Email)
		assert.Equal(s.T(), authResponse.User.Name, userResponse.Name)
	})

	s.Run("registration validation failures", func() {
		testCases := []struct {
			name        string
			request     models.RegisterRequest
			expectedErr string
		}{
			{
				name:        "empty email",
				request:     s.Factory.InvalidRegisterRequestEmptyEmail(),
				expectedErr: "email",
			},
			{
				name:        "weak password",
				request:     s.Factory.InvalidRegisterRequestWeakPassword(),
				expectedErr: "password",
			},
			{
				name: "invalid email format",
				request: models.RegisterRequest{
					Email:    "invalid-email",
					Password: "ValidPassword123!",
					Name:     "Test User",
				},
				expectedErr: "email",
			},
			{
				name: "empty name",
				request: models.RegisterRequest{
					Email:    "test@example.com",
					Password: "ValidPassword123!",
					Name:     "",
				},
				expectedErr: "name",
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				resp := s.HTTP.POST(s.T(), "/api/v1/auth/register", tc.request).ExpectBadRequest()

				var errorResponse models.ErrorResponse
				resp.UnmarshalResponse(&errorResponse)

				assert.Contains(s.T(), strings.ToLower(errorResponse.Error), strings.ToLower(tc.expectedErr))
			})
		}
	})

	s.Run("duplicate email registration", func() {
		// Register first user
		registerReq := s.Factory.ValidRegisterRequestWithEmail("duplicate@example.com")
		s.HTTP.POST(s.T(), "/api/v1/auth/register", registerReq).ExpectCreated()

		// Try to register again with same email
		duplicateReq := s.Factory.ValidRegisterRequestWithEmail("duplicate@example.com")
		resp := s.HTTP.POST(s.T(), "/api/v1/auth/register", duplicateReq).ExpectStatus(409) // Conflict

		var errorResponse models.ErrorResponse
		resp.UnmarshalResponse(&errorResponse)

		assert.Contains(s.T(), strings.ToLower(errorResponse.Error), "already exists")
	})
}

// TestUserLoginFlow tests the complete user login workflow
func (s *AuthIntegrationSuite) TestUserLoginFlow() {
	s.Run("successful login with valid credentials", func() {
		// First register a user
		registerReq := s.Factory.ValidRegisterRequestWithEmail("login@example.com")
		s.HTTP.POST(s.T(), "/api/v1/auth/register", registerReq).ExpectCreated()

		// Now login
		loginReq := s.Factory.ValidLoginRequestWithCredentials(registerReq.Email, registerReq.Password)
		resp := s.HTTP.POST(s.T(), "/api/v1/auth/login", loginReq).ExpectOK().ExpectJSON()

		var authResponse models.AuthResponse
		resp.UnmarshalResponse(&authResponse)

		// Validate response structure
		testutil.ValidateAuthTokens(s.T(), &authResponse)
		assert.Equal(s.T(), registerReq.Email, authResponse.User.Email)
		assert.Equal(s.T(), registerReq.Name, authResponse.User.Name)

		// Verify tokens are valid
		s.validateJWTToken(authResponse.AccessToken, authResponse.User.ID.String())
		s.validateJWTToken(authResponse.RefreshToken, authResponse.User.ID.String())
	})

	s.Run("login with invalid credentials", func() {
		// Register a user
		registerReq := s.Factory.ValidRegisterRequestWithEmail("invalid@example.com")
		s.HTTP.POST(s.T(), "/api/v1/auth/register", registerReq).ExpectCreated()

		testCases := []struct {
			name           string
			email          string
			password       string
			expectedErr    string
			expectedStatus int
		}{
			{
				name:           "wrong password",
				email:          registerReq.Email,
				password:       "WrongPassword123!",
				expectedErr:    "invalid email or password",
				expectedStatus: 401,
			},
			{
				name:           "non-existent email",
				email:          "nonexistent@example.com",
				password:       registerReq.Password,
				expectedErr:    "invalid email or password",
				expectedStatus: 401,
			},
			{
				name:           "empty email",
				email:          "",
				password:       registerReq.Password,
				expectedErr:    "email",
				expectedStatus: 400,
			},
			{
				name:           "empty password",
				email:          registerReq.Email,
				password:       "",
				expectedErr:    "password",
				expectedStatus: 400,
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				loginReq := s.Factory.ValidLoginRequestWithCredentials(tc.email, tc.password)
				resp := s.HTTP.POST(s.T(), "/api/v1/auth/login", loginReq).ExpectStatus(tc.expectedStatus)

				var errorResponse models.ErrorResponse
				resp.UnmarshalResponse(&errorResponse)

				assert.Contains(s.T(), strings.ToLower(errorResponse.Error), strings.ToLower(tc.expectedErr))
			})
		}
	})
}

// TestTokenRefreshFlow tests the JWT token refresh workflow
func (s *AuthIntegrationSuite) TestTokenRefreshFlow() {
	s.Run("successful token refresh", func() {
		// Register and get initial tokens
		registerReq := s.Factory.ValidRegisterRequestWithEmail("refresh@example.com")
		registerResp := s.HTTP.POST(s.T(), "/api/v1/auth/register", registerReq).ExpectCreated()

		var initialAuth models.AuthResponse
		registerResp.UnmarshalResponse(&initialAuth)

		// Wait a moment to ensure timestamp difference
		time.Sleep(1 * time.Second)

		// Refresh tokens
		refreshReq := s.Factory.ValidRefreshTokenRequest(initialAuth.RefreshToken)
		refreshResp := s.HTTP.POST(s.T(), "/api/v1/auth/refresh", refreshReq).ExpectOK().ExpectJSON()

		var refreshAuth models.AuthResponse
		refreshResp.UnmarshalResponse(&refreshAuth)

		// Validate new tokens
		testutil.ValidateAuthTokens(s.T(), &refreshAuth)

		// New access token should be different
		assert.NotEqual(s.T(), initialAuth.AccessToken, refreshAuth.AccessToken)

		// User info should be the same
		assert.Equal(s.T(), initialAuth.User.ID, refreshAuth.User.ID)
		assert.Equal(s.T(), initialAuth.User.Email, refreshAuth.User.Email)

		// Both old and new access tokens should work (until old one expires)
		oldHeaders := s.Factory.GetAuthHeaders(initialAuth.AccessToken)
		oldMeReq := testutil.NewRequest("GET", "/api/v1/auth/me")
		for key, value := range oldHeaders {
			oldMeReq.WithHeader(key, value)
		}
		s.HTTP.Do(s.T(), oldMeReq).ExpectOK()

		newHeaders := s.Factory.GetAuthHeaders(refreshAuth.AccessToken)
		newMeReq := testutil.NewRequest("GET", "/api/v1/auth/me")
		for key, value := range newHeaders {
			newMeReq.WithHeader(key, value)
		}
		s.HTTP.Do(s.T(), newMeReq).ExpectOK()

		// Verify JWT token validity
		s.validateJWTToken(refreshAuth.AccessToken, refreshAuth.User.ID.String())
		s.validateJWTToken(refreshAuth.RefreshToken, refreshAuth.User.ID.String())
	})

	s.Run("refresh with invalid token", func() {
		testCases := []struct {
			name         string
			refreshToken string
			expectedErr  string
		}{
			{
				name:         "empty refresh token",
				refreshToken: "",
				expectedErr:  "refresh token",
			},
			{
				name:         "invalid refresh token",
				refreshToken: "invalid-token",
				expectedErr:  "invalid",
			},
			{
				name:         "malformed JWT",
				refreshToken: "not.a.jwt",
				expectedErr:  "invalid",
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				refreshReq := s.Factory.ValidRefreshTokenRequest(tc.refreshToken)
				resp := s.HTTP.POST(s.T(), "/api/v1/auth/refresh", refreshReq).ExpectStatus(401)

				var errorResponse models.ErrorResponse
				resp.UnmarshalResponse(&errorResponse)

				assert.Contains(s.T(), strings.ToLower(errorResponse.Error), strings.ToLower(tc.expectedErr))
			})
		}
	})
}

// TestProtectedEndpointAccess tests access control for protected endpoints
func (s *AuthIntegrationSuite) TestProtectedEndpointAccess() {
	s.Run("access without authentication", func() {
		protectedEndpoints := []struct {
			method string
			path   string
		}{
			{"GET", "/api/v1/auth/me"},
			{"GET", "/api/v1/tasks"},
			{"POST", "/api/v1/tasks"},
			{"GET", "/api/v1/tasks/123e4567-e89b-12d3-a456-426614174000"},
		}

		for _, endpoint := range protectedEndpoints {
			s.Run(endpoint.method+" "+endpoint.path, func() {
				req := testutil.NewRequest(endpoint.method, endpoint.path)
				s.HTTP.Do(s.T(), req).ExpectUnauthorized()
			})
		}
	})

	s.Run("access with invalid token", func() {
		invalidTokens := []struct {
			name  string
			token string
		}{
			{"empty token", ""},
			{"invalid format", "invalid-token"},
			{"malformed JWT", "not.a.jwt.token"},
			{"expired token", s.createExpiredToken()},
		}

		for _, tokenCase := range invalidTokens {
			s.Run(tokenCase.name, func() {
				headers := s.Factory.GetAuthHeaders(tokenCase.token)
				req := testutil.NewRequest("GET", "/api/v1/auth/me")
				for key, value := range headers {
					req.WithHeader(key, value)
				}

				s.HTTP.Do(s.T(), req).ExpectUnauthorized()
			})
		}
	})

	s.Run("access with valid token", func() {
		// Register user and get token
		registerReq := s.Factory.ValidRegisterRequestWithEmail("protected@example.com")
		registerResp := s.HTTP.POST(s.T(), "/api/v1/auth/register", registerReq).ExpectCreated()

		var authResponse models.AuthResponse
		registerResp.UnmarshalResponse(&authResponse)

		// Access protected endpoint
		headers := s.Factory.GetAuthHeaders(authResponse.AccessToken)
		req := testutil.NewRequest("GET", "/api/v1/auth/me")
		for key, value := range headers {
			req.WithHeader(key, value)
		}

		resp := s.HTTP.Do(s.T(), req).ExpectOK().ExpectJSON()

		var meResponseWrapper map[string]models.UserResponse
		resp.UnmarshalResponse(&meResponseWrapper)

		userResponse := meResponseWrapper["user"]

		assert.Equal(s.T(), authResponse.User.ID, userResponse.ID)
		assert.Equal(s.T(), authResponse.User.Email, userResponse.Email)
	})
}

// TestLogoutFlow tests the logout workflow
func (s *AuthIntegrationSuite) TestLogoutFlow() {
	s.Run("successful logout", func() {
		// Register user and get token
		registerReq := s.Factory.ValidRegisterRequestWithEmail("logout@example.com")
		registerResp := s.HTTP.POST(s.T(), "/api/v1/auth/register", registerReq).ExpectCreated()

		var authResponse models.AuthResponse
		registerResp.UnmarshalResponse(&authResponse)

		// Verify token works before logout
		headers := s.Factory.GetAuthHeaders(authResponse.AccessToken)
		preLogoutReq := testutil.NewRequest("GET", "/api/v1/auth/me")
		for key, value := range headers {
			preLogoutReq.WithHeader(key, value)
		}
		s.HTTP.Do(s.T(), preLogoutReq).ExpectOK()

		// Logout
		logoutReq := testutil.NewRequest("POST", "/api/v1/auth/logout")
		for key, value := range headers {
			logoutReq.WithHeader(key, value)
		}
		s.HTTP.Do(s.T(), logoutReq).ExpectOK()

		// Note: In a real implementation with token blacklisting,
		// the token might be invalidated. For now, we just verify
		// the logout endpoint works.
	})

	s.Run("logout without authentication", func() {
		// JWT logout is typically client-side, so the endpoint allows logout without auth
		s.HTTP.POST(s.T(), "/api/v1/auth/logout", nil).ExpectOK()
	})
}

// TestConcurrentAuthentication tests concurrent authentication scenarios
func (s *AuthIntegrationSuite) TestConcurrentAuthentication() {
	s.Run("concurrent user registrations", func() {
		numUsers := 5
		results := make(chan error, numUsers)

		for i := 0; i < numUsers; i++ {
			go func(index int) {
				registerReq := s.Factory.ValidRegisterRequestWithEmail(fmt.Sprintf("concurrent%d@example.com", index))
				resp := s.HTTP.POST(s.T(), "/api/v1/auth/register", registerReq)

				if resp.Code != 201 {
					results <- fmt.Errorf("user %d registration failed with status %d", index, resp.Code)
				} else {
					results <- nil
				}
			}(i)
		}

		// Wait for all registrations to complete
		for i := 0; i < numUsers; i++ {
			err := <-results
			assert.NoError(s.T(), err)
		}
	})

	s.Run("concurrent logins for same user", func() {
		// Register a user
		registerReq := s.Factory.ValidRegisterRequestWithEmail("multilogin@example.com")
		s.HTTP.POST(s.T(), "/api/v1/auth/register", registerReq).ExpectCreated()

		// Attempt concurrent logins
		numLogins := 3
		results := make(chan models.AuthResponse, numLogins)

		for i := 0; i < numLogins; i++ {
			go func() {
				loginReq := s.Factory.ValidLoginRequestWithCredentials(registerReq.Email, registerReq.Password)
				resp := s.HTTP.POST(s.T(), "/api/v1/auth/login", loginReq)

				if resp.Code == 200 {
					var authResponse models.AuthResponse
					resp.UnmarshalResponse(&authResponse)
					results <- authResponse
				}
			}()
		}

		// Verify all logins succeeded and returned valid tokens
		validLogins := 0
		for i := 0; i < numLogins; i++ {
			select {
			case authResp := <-results:
				testutil.ValidateAuthTokens(s.T(), &authResp)
				validLogins++
			case <-time.After(5 * time.Second):
				s.T().Error("Login timed out")
			}
		}

		assert.Equal(s.T(), numLogins, validLogins)
	})
}

// TestTokenExpiryScenarios tests token expiry handling
func (s *AuthIntegrationSuite) TestTokenExpiryScenarios() {
	s.Run("access token expiry", func() {
		if testing.Short() {
			s.T().Skip("Skipping token expiry test in short mode")
		}

		// Register user with short-lived tokens
		registerReq := s.Factory.ValidRegisterRequestWithEmail("expiry@example.com")
		registerResp := s.HTTP.POST(s.T(), "/api/v1/auth/register", registerReq).ExpectCreated()

		var authResponse models.AuthResponse
		registerResp.UnmarshalResponse(&authResponse)

		// Verify token works initially
		headers := s.Factory.GetAuthHeaders(authResponse.AccessToken)
		req := testutil.NewRequest("GET", "/api/v1/auth/me")
		for key, value := range headers {
			req.WithHeader(key, value)
		}
		s.HTTP.Do(s.T(), req).ExpectOK()

		// Wait for token to expire (our test config uses 30 seconds)
		s.T().Log("Waiting for access token to expire...")
		time.Sleep(35 * time.Second)

		// Token should now be expired
		expiredReq := testutil.NewRequest("GET", "/api/v1/auth/me")
		for key, value := range headers {
			expiredReq.WithHeader(key, value)
		}
		s.HTTP.Do(s.T(), expiredReq).ExpectUnauthorized()

		// But refresh token should still work
		refreshReq := s.Factory.ValidRefreshTokenRequest(authResponse.RefreshToken)
		refreshResp := s.HTTP.POST(s.T(), "/api/v1/auth/refresh", refreshReq).ExpectOK()

		var newAuth models.AuthResponse
		refreshResp.UnmarshalResponse(&newAuth)

		// New token should work
		newHeaders := s.Factory.GetAuthHeaders(newAuth.AccessToken)
		newReq := testutil.NewRequest("GET", "/api/v1/auth/me")
		for key, value := range newHeaders {
			newReq.WithHeader(key, value)
		}
		s.HTTP.Do(s.T(), newReq).ExpectOK()
	})
}

// validateJWTToken validates that a JWT token is properly formatted and contains expected claims
func (s *AuthIntegrationSuite) validateJWTToken(tokenString, expectedUserID string) {
	// Parse and validate the token
	claims, err := s.JWTService.ValidateToken(tokenString)
	require.NoError(s.T(), err, "Token should be valid")
	require.NotNil(s.T(), claims, "Claims should not be nil")

	// Verify standard claims
	assert.Equal(s.T(), s.Config.JWT.Issuer, claims.Issuer, "Issuer should match config")
	assert.Equal(s.T(), s.Config.JWT.Audience, claims.Audience[0], "Audience should match config")
	assert.Equal(s.T(), expectedUserID, claims.Subject, "Subject should match user ID")

	// Verify timestamps
	assert.NotNil(s.T(), claims.IssuedAt, "Issued at should be present")
	assert.NotNil(s.T(), claims.ExpiresAt, "Expiry should be present")

	// Verify custom claims
	assert.Equal(s.T(), expectedUserID, claims.UserID.String(), "UserID should match")
	assert.NotEmpty(s.T(), claims.Email, "Email should be present")
}

// createExpiredToken creates an intentionally expired JWT token for testing
func (s *AuthIntegrationSuite) createExpiredToken() string {
	// Create a token with past expiry time
	expiredConfig := s.Config.JWT
	expiredConfig.AccessTokenDuration = -1 * time.Hour // Already expired

	expiredJWTService := auth.NewJWTService(&expiredConfig)

	// Create a dummy user for testing
	dummyUser := testutil.NewUserFactory().Build()
	token, _ := expiredJWTService.GenerateTokenPair(dummyUser)

	return token.AccessToken
}
