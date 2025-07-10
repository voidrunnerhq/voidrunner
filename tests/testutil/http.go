package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/auth"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// HTTPHelper provides utilities for HTTP testing
type HTTPHelper struct {
	Router      *gin.Engine
	AuthService *auth.Service
}

// NewHTTPHelper creates a new HTTP helper for testing
func NewHTTPHelper(router *gin.Engine, authService *auth.Service) *HTTPHelper {
	gin.SetMode(gin.TestMode)

	return &HTTPHelper{
		Router:      router,
		AuthService: authService,
	}
}

// Request represents an HTTP request for testing
type Request struct {
	Method  string
	URL     string
	Body    interface{}
	Headers map[string]string
	Auth    *AuthContext
}

// AuthContext contains authentication information for requests
type AuthContext struct {
	User        *models.User
	AccessToken string
}

// Response wraps httptest.ResponseRecorder with helper methods
type Response struct {
	*httptest.ResponseRecorder
	t *testing.T
}

// NewRequest creates a new test request
func NewRequest(method, url string) *Request {
	return &Request{
		Method:  method,
		URL:     url,
		Headers: make(map[string]string),
	}
}

// WithBody sets the request body
func (r *Request) WithBody(body interface{}) *Request {
	r.Body = body
	return r
}

// WithHeader adds a header to the request
func (r *Request) WithHeader(key, value string) *Request {
	r.Headers[key] = value
	return r
}

// WithAuth sets authentication for the request
func (r *Request) WithAuth(auth *AuthContext) *Request {
	r.Auth = auth
	return r
}

// WithJSONContentType adds JSON content type header
func (r *Request) WithJSONContentType() *Request {
	return r.WithHeader("Content-Type", "application/json")
}

// Do executes the HTTP request and returns a response wrapper
func (h *HTTPHelper) Do(t *testing.T, req *Request) *Response {
	t.Helper()

	// Prepare request body
	var bodyReader *bytes.Reader
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		require.NoError(t, err, "failed to marshal request body")
		bodyReader = bytes.NewReader(bodyBytes)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	// Create HTTP request
	httpReq, err := http.NewRequest(req.Method, req.URL, bodyReader)
	require.NoError(t, err, "failed to create HTTP request")

	// Add headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Add authentication if provided
	if req.Auth != nil && req.Auth.AccessToken != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", req.Auth.AccessToken))
	}

	// Execute request
	recorder := httptest.NewRecorder()
	h.Router.ServeHTTP(recorder, httpReq)

	return &Response{
		ResponseRecorder: recorder,
		t:                t,
	}
}

// GET creates and executes a GET request
func (h *HTTPHelper) GET(t *testing.T, url string) *Response {
	return h.Do(t, NewRequest("GET", url))
}

// POST creates and executes a POST request
func (h *HTTPHelper) POST(t *testing.T, url string, body interface{}) *Response {
	return h.Do(t, NewRequest("POST", url).WithBody(body).WithJSONContentType())
}

// PUT creates and executes a PUT request
func (h *HTTPHelper) PUT(t *testing.T, url string, body interface{}) *Response {
	return h.Do(t, NewRequest("PUT", url).WithBody(body).WithJSONContentType())
}

// DELETE creates and executes a DELETE request
func (h *HTTPHelper) DELETE(t *testing.T, url string) *Response {
	return h.Do(t, NewRequest("DELETE", url))
}

// PATCH creates and executes a PATCH request
func (h *HTTPHelper) PATCH(t *testing.T, url string, body interface{}) *Response {
	return h.Do(t, NewRequest("PATCH", url).WithBody(body).WithJSONContentType())
}

// ExpectStatus asserts that the response has the expected status code
func (r *Response) ExpectStatus(expectedStatus int) *Response {
	r.t.Helper()
	if r.Code != expectedStatus {
		r.t.Errorf("Expected status %d, got %d. Response body: %s", expectedStatus, r.Code, r.Body.String())
	}
	return r
}

// ExpectOK asserts that the response status is 200 OK
func (r *Response) ExpectOK() *Response {
	return r.ExpectStatus(http.StatusOK)
}

// ExpectCreated asserts that the response status is 201 Created
func (r *Response) ExpectCreated() *Response {
	return r.ExpectStatus(http.StatusCreated)
}

// ExpectBadRequest asserts that the response status is 400 Bad Request
func (r *Response) ExpectBadRequest() *Response {
	return r.ExpectStatus(http.StatusBadRequest)
}

// ExpectUnauthorized asserts that the response status is 401 Unauthorized
func (r *Response) ExpectUnauthorized() *Response {
	return r.ExpectStatus(http.StatusUnauthorized)
}

// ExpectForbidden asserts that the response status is 403 Forbidden
func (r *Response) ExpectForbidden() *Response {
	return r.ExpectStatus(http.StatusForbidden)
}

// ExpectNotFound asserts that the response status is 404 Not Found
func (r *Response) ExpectNotFound() *Response {
	return r.ExpectStatus(http.StatusNotFound)
}

// ExpectInternalServerError asserts that the response status is 500 Internal Server Error
func (r *Response) ExpectInternalServerError() *Response {
	return r.ExpectStatus(http.StatusInternalServerError)
}

// ExpectJSON asserts that the response has JSON content type
func (r *Response) ExpectJSON() *Response {
	r.t.Helper()
	contentType := r.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		r.t.Errorf("Expected JSON content type, got: %s", contentType)
	}
	return r
}

// UnmarshalResponse unmarshals the response body into the provided struct
func (r *Response) UnmarshalResponse(v interface{}) *Response {
	r.t.Helper()
	err := json.Unmarshal(r.Body.Bytes(), v)
	require.NoError(r.t, err, "failed to unmarshal response JSON")
	return r
}

// ExpectBodyContains asserts that the response body contains the expected string
func (r *Response) ExpectBodyContains(expected string) *Response {
	r.t.Helper()
	body := r.Body.String()
	if !strings.Contains(body, expected) {
		r.t.Errorf("Expected response body to contain '%s', but got: %s", expected, body)
	}
	return r
}

// ExpectError asserts that the response contains an error with the expected message
func (r *Response) ExpectError(expectedMessage string) *Response {
	r.t.Helper()
	var errorResp map[string]interface{}
	r.UnmarshalResponse(&errorResp)

	if message, ok := errorResp["error"].(string); ok {
		if !strings.Contains(message, expectedMessage) {
			r.t.Errorf("Expected error message to contain '%s', but got: %s", expectedMessage, message)
		}
	} else {
		r.t.Errorf("Expected error response with message containing '%s', but got: %v", expectedMessage, errorResp)
	}
	return r
}

// PrintBody prints the response body for debugging
func (r *Response) PrintBody() *Response {
	r.t.Logf("Response body: %s", r.Body.String())
	return r
}

// AuthHelper provides authentication utilities for testing
type AuthHelper struct {
	authService *auth.Service
}

// NewAuthHelper creates a new auth helper
func NewAuthHelper(authService *auth.Service) *AuthHelper {
	return &AuthHelper{
		authService: authService,
	}
}

// CreateAuthContext creates an authentication context for a user
func (a *AuthHelper) CreateAuthContext(t *testing.T, user *models.User) *AuthContext {
	t.Helper()

	// This is a simplified version - in real tests, we'd need access to the JWT service
	// For now, return a context that can be used with pre-registered users
	return &AuthContext{
		User:        user,
		AccessToken: "test-access-token", // This would need to be a real token in integration tests
	}
}

// LoginUser simulates user login and returns auth context
func (h *HTTPHelper) LoginUser(t *testing.T, email, password string) *AuthContext {
	t.Helper()

	loginReq := models.LoginRequest{
		Email:    email,
		Password: password,
	}

	resp := h.POST(t, "/api/v1/auth/login", loginReq).ExpectOK()

	var authResp models.AuthResponse
	resp.UnmarshalResponse(&authResp)

	// Convert UserResponse to User for auth context
	user := &models.User{
		BaseModel: models.BaseModel{
			ID: authResp.User.ID,
		},
		Email: authResp.User.Email,
		Name:  authResp.User.Name,
	}

	return &AuthContext{
		User:        user,
		AccessToken: authResp.AccessToken,
	}
}

// RegisterUser creates a new user and returns auth context
func (h *HTTPHelper) RegisterUser(t *testing.T, email, password, name string) *AuthContext {
	t.Helper()

	registerReq := models.RegisterRequest{
		Email:    email,
		Password: password,
		Name:     name,
	}

	resp := h.POST(t, "/api/v1/auth/register", registerReq).ExpectCreated()

	var authResp models.AuthResponse
	resp.UnmarshalResponse(&authResp)

	// Convert UserResponse to User for auth context
	user := &models.User{
		BaseModel: models.BaseModel{
			ID: authResp.User.ID,
		},
		Email: authResp.User.Email,
		Name:  authResp.User.Name,
	}

	return &AuthContext{
		User:        user,
		AccessToken: authResp.AccessToken,
	}
}

// CreateAuthenticatedRequest creates a request with authentication
func (h *HTTPHelper) CreateAuthenticatedRequest(method, url string, auth *AuthContext) *Request {
	return NewRequest(method, url).WithAuth(auth).WithJSONContentType()
}

// AuthenticatedGET performs a GET request with authentication
func (h *HTTPHelper) AuthenticatedGET(t *testing.T, url string, auth *AuthContext) *Response {
	return h.Do(t, h.CreateAuthenticatedRequest("GET", url, auth))
}

// AuthenticatedPOST performs a POST request with authentication
func (h *HTTPHelper) AuthenticatedPOST(t *testing.T, url string, body interface{}, auth *AuthContext) *Response {
	return h.Do(t, h.CreateAuthenticatedRequest("POST", url, auth).WithBody(body))
}

// AuthenticatedPUT performs a PUT request with authentication
func (h *HTTPHelper) AuthenticatedPUT(t *testing.T, url string, body interface{}, auth *AuthContext) *Response {
	return h.Do(t, h.CreateAuthenticatedRequest("PUT", url, auth).WithBody(body))
}

// AuthenticatedDELETE performs a DELETE request with authentication
func (h *HTTPHelper) AuthenticatedDELETE(t *testing.T, url string, auth *AuthContext) *Response {
	return h.Do(t, h.CreateAuthenticatedRequest("DELETE", url, auth))
}

// ValidateAuthTokens validates that auth response contains valid tokens
func ValidateAuthTokens(t *testing.T, authResp *models.AuthResponse) {
	t.Helper()

	require.NotEmpty(t, authResp.AccessToken, "access token should not be empty")
	require.NotEmpty(t, authResp.RefreshToken, "refresh token should not be empty")
	require.Equal(t, "Bearer", authResp.TokenType, "token type should be Bearer")
	require.Greater(t, authResp.ExpiresIn, int64(0), "expires in should be positive")
	require.NotEmpty(t, authResp.User.ID, "user ID should not be empty")
	require.NotEmpty(t, authResp.User.Email, "user email should not be empty")
}
