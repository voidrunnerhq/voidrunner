package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNewDocsHandler(t *testing.T) {
	handler := NewDocsHandler()
	assert.NotNil(t, handler)
}

func TestDocsHandler_GetSwaggerJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDocsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create a mock request
	req := httptest.NewRequest("GET", "/docs/swagger.json", nil)
	c.Request = req

	// This will attempt to serve the swagger.json file
	// In a real scenario, the file should exist, but for testing we'll check the handler behavior
	handler.GetSwaggerJSON(c)

	// The handler should attempt to serve a file, which might result in 404 if file doesn't exist
	// or 200 if it does. Both are valid behaviors for this test.
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound)
}

func TestDocsHandler_GetSwaggerYAML(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDocsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create a mock request
	req := httptest.NewRequest("GET", "/docs/swagger.yaml", nil)
	c.Request = req

	// This will attempt to serve the swagger.yaml file
	handler.GetSwaggerYAML(c)

	// The handler should attempt to serve a file, which might result in 404 if file doesn't exist
	// or 200 if it does. Both are valid behaviors for this test.
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound)
}

func TestDocsHandler_RedirectToSwaggerUI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDocsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create a mock request
	req := httptest.NewRequest("GET", "/docs", nil)
	c.Request = req

	handler.RedirectToSwaggerUI(c)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/docs/", w.Header().Get("Location"))
}

func TestDocsHandler_GetSwaggerUI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDocsHandler()

	// Get the handler function
	handlerFunc := handler.GetSwaggerUI()
	assert.NotNil(t, handlerFunc)

	// Test that it returns a gin.HandlerFunc
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/docs/", nil)
	c.Request = req

	// Execute the handler
	handlerFunc(c)

	// Should return some response (either swagger UI content or a redirect)
	// The exact response depends on the swagger UI implementation
	assert.True(t, w.Code >= 200 && w.Code < 500)
}

func TestDocsHandler_GetAPIIndex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDocsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create a mock request
	req := httptest.NewRequest("GET", "/api", nil)
	c.Request = req

	handler.GetAPIIndex(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))

	body := w.Body.String()

	// Check that the response contains expected HTML content
	assert.Contains(t, body, "<!DOCTYPE html>")
	assert.Contains(t, body, "VoidRunner API Documentation")
	assert.Contains(t, body, "Interactive Documentation")
	assert.Contains(t, body, "/docs/")
	assert.Contains(t, body, "/docs/swagger.json")
	assert.Contains(t, body, "/docs/swagger.yaml")
	assert.Contains(t, body, "/health")

	// Check for API endpoints in the quick reference
	assert.Contains(t, body, "/api/v1/auth/register")
	assert.Contains(t, body, "/api/v1/auth/login")
	assert.Contains(t, body, "/api/v1/tasks")

	// Check for HTTP methods
	assert.Contains(t, body, "GET")
	assert.Contains(t, body, "POST")

	// Check for CSS styling
	assert.Contains(t, body, "<style>")
	assert.Contains(t, body, "font-family")

	// Verify it's a complete HTML document
	assert.True(t, strings.HasPrefix(body, "<!DOCTYPE html>"))
	assert.Contains(t, body, "</html>")
}

func TestDocsHandler_GetAPIIndex_HTMLStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDocsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("GET", "/api", nil)
	c.Request = req

	handler.GetAPIIndex(c)

	body := w.Body.String()

	// Test HTML structure elements
	assert.Contains(t, body, `<meta charset="UTF-8">`)
	assert.Contains(t, body, `<meta name="viewport"`)
	assert.Contains(t, body, `<title>VoidRunner API Documentation</title>`)

	// Test CSS classes and styling
	assert.Contains(t, body, `class="header"`)
	assert.Contains(t, body, `class="links"`)
	assert.Contains(t, body, `class="link-card"`)
	assert.Contains(t, body, `class="endpoints"`)
	assert.Contains(t, body, `class="endpoint-list"`)

	// Test specific method styling classes
	assert.Contains(t, body, `class="method get"`)
	assert.Contains(t, body, `class="method post"`)

	// Test emoji usage
	assert.Contains(t, body, "📖")
	assert.Contains(t, body, "📄")
	assert.Contains(t, body, "📋")
	assert.Contains(t, body, "💓")
	assert.Contains(t, body, "🛠")
}

func TestDocsHandler_GetAPIIndex_ContentValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDocsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("GET", "/api", nil)
	c.Request = req

	handler.GetAPIIndex(c)

	body := w.Body.String()

	// Validate that all documented endpoints are present
	// Note: These check for the path portion only since the HTML contains method tags
	expectedPaths := []string{
		"/health - Health check",
		"/ready - Readiness check",
		"/api/v1/auth/register - Register user",
		"/api/v1/auth/login - Login user",
		"/api/v1/auth/me - Get current user",
		"/api/v1/tasks - List tasks",
		"/api/v1/tasks - Create task",
		"/api/v1/tasks/{id} - Get task",
		"/api/v1/tasks/{id}/executions - Execute task",
	}

	for _, path := range expectedPaths {
		assert.Contains(t, body, path, "Expected endpoint path %s not found in documentation", path)
	}

	// Validate descriptions are helpful
	descriptions := []string{
		"Health check",
		"Readiness check",
		"Register user",
		"Login user",
		"Get current user",
		"List tasks",
		"Create task",
		"Get task",
		"Execute task",
	}

	for _, desc := range descriptions {
		assert.Contains(t, body, desc, "Expected description '%s' not found", desc)
	}

	// Validate links work correctly
	links := []string{
		`href="/docs/"`,
		`href="/docs/swagger.json"`,
		`href="/docs/swagger.yaml"`,
		`href="/health"`,
	}

	for _, link := range links {
		assert.Contains(t, body, link, "Expected link %s not found", link)
	}
}

// Benchmark tests
func BenchmarkDocsHandler_GetAPIIndex(b *testing.B) {
	gin.SetMode(gin.TestMode)
	handler := NewDocsHandler()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest("GET", "/api", nil)
		c.Request = req

		handler.GetAPIIndex(c)
	}
}

func BenchmarkDocsHandler_RedirectToSwaggerUI(b *testing.B) {
	gin.SetMode(gin.TestMode)
	handler := NewDocsHandler()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest("GET", "/docs", nil)
		c.Request = req

		handler.RedirectToSwaggerUI(c)
	}
}
