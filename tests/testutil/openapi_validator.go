package testutil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OpenAPIValidator provides validation against OpenAPI specification
type OpenAPIValidator struct {
	spec *OpenAPISpec
}

// OpenAPISpec represents a simplified OpenAPI specification for validation
type OpenAPISpec struct {
	Paths map[string]map[string]*OperationSpec `json:"paths"`
}

// OperationSpec defines expected response structure for an API operation
type OperationSpec struct {
	Responses map[string]*ResponseSpec `json:"responses"`
}

// ResponseSpec defines the expected response structure
type ResponseSpec struct {
	Description string                 `json:"description"`
	Content     map[string]*MediaType  `json:"content"`
	Headers     map[string]*HeaderSpec `json:"headers"`
}

// MediaType defines the expected media type structure
type MediaType struct {
	Schema *SchemaSpec `json:"schema"`
}

// SchemaSpec defines expected JSON schema properties
type SchemaSpec struct {
	Type       string                 `json:"type"`
	Properties map[string]*SchemaSpec `json:"properties"`
	Required   []string               `json:"required"`
	Items      *SchemaSpec            `json:"items"`
}

// HeaderSpec defines expected header structure
type HeaderSpec struct {
	Description string      `json:"description"`
	Schema      *SchemaSpec `json:"schema"`
}

// NewOpenAPIValidator creates a new OpenAPI validator.
// Each validator instance has its own spec copy for thread safety.
func NewOpenAPIValidator() *OpenAPIValidator {
	return &OpenAPIValidator{
		spec: loadOpenAPISpec(),
	}
}

// GetSpec returns the OpenAPI specification
func (v *OpenAPIValidator) GetSpec() *OpenAPISpec {
	return v.spec
}

// loadOpenAPISpec loads and parses the OpenAPI specification.
// This function creates a new spec instance each time to ensure thread safety.
// In concurrent testing scenarios, each validator gets its own spec copy.
func loadOpenAPISpec() *OpenAPISpec {
	// For now, return a basic spec structure that covers the main API endpoints
	// In a full implementation, this would parse the actual openapi.yaml file
	// Note: If file loading is added, consider using sync.Once for caching while
	// maintaining thread safety.
	return &OpenAPISpec{
		Paths: map[string]map[string]*OperationSpec{
			"/api/v1/auth/register": {
				"post": {
					Responses: map[string]*ResponseSpec{
						"201": {
							Description: "User registered successfully",
							Content: map[string]*MediaType{
								"application/json": {
									Schema: &SchemaSpec{
										Type: "object",
										Properties: map[string]*SchemaSpec{
											"user": {
												Type: "object",
												Properties: map[string]*SchemaSpec{
													"id":         {Type: "string"},
													"email":      {Type: "string"},
													"name":       {Type: "string"},
													"created_at": {Type: "string"},
													"updated_at": {Type: "string"},
												},
												Required: []string{"id", "email", "name", "created_at", "updated_at"},
											},
											"access_token":  {Type: "string"},
											"refresh_token": {Type: "string"},
											"token_type":    {Type: "string"},
											"expires_in":    {Type: "integer"},
										},
										Required: []string{"user", "access_token", "refresh_token", "token_type", "expires_in"},
									},
								},
							},
						},
						"400": {
							Description: "Validation error",
							Content: map[string]*MediaType{
								"application/json": {
									Schema: &SchemaSpec{
										Type: "object",
										Properties: map[string]*SchemaSpec{
											"error": {Type: "string"},
											"validation_errors": {
												Type:  "array",
												Items: &SchemaSpec{Type: "string"},
											},
										},
										Required: []string{"error"},
									},
								},
							},
						},
					},
				},
			},
			"/api/v1/auth/login": {
				"post": {
					Responses: map[string]*ResponseSpec{
						"200": {
							Description: "Login successful",
							Content: map[string]*MediaType{
								"application/json": {
									Schema: &SchemaSpec{
										Type: "object",
										Properties: map[string]*SchemaSpec{
											"user": {
												Type: "object",
												Properties: map[string]*SchemaSpec{
													"id":         {Type: "string"},
													"email":      {Type: "string"},
													"name":       {Type: "string"},
													"created_at": {Type: "string"},
													"updated_at": {Type: "string"},
												},
												Required: []string{"id", "email", "name", "created_at", "updated_at"},
											},
											"access_token":  {Type: "string"},
											"refresh_token": {Type: "string"},
											"token_type":    {Type: "string"},
											"expires_in":    {Type: "integer"},
										},
										Required: []string{"user", "access_token", "refresh_token", "token_type", "expires_in"},
									},
								},
							},
						},
					},
				},
			},
			"/api/v1/tasks": {
				"get": {
					Responses: map[string]*ResponseSpec{
						"200": {
							Description: "List of tasks",
							Content: map[string]*MediaType{
								"application/json": {
									Schema: &SchemaSpec{
										Type: "object",
										Properties: map[string]*SchemaSpec{
											"tasks": {
												Type: "array",
												Items: &SchemaSpec{
													Type: "object",
													Properties: map[string]*SchemaSpec{
														"id":             {Type: "string"},
														"name":           {Type: "string"},
														"status":         {Type: "string"},
														"script_content": {Type: "string"},
														"script_type":    {Type: "string"},
														"created_at":     {Type: "string"},
													},
													Required: []string{"id", "name", "status", "script_content", "script_type", "created_at"},
												},
											},
											"total":  {Type: "integer"},
											"limit":  {Type: "integer"},
											"offset": {Type: "integer"},
										},
										Required: []string{"tasks", "total", "limit", "offset"},
									},
								},
							},
						},
						"401": {
							Description: "Unauthorized access",
							Content: map[string]*MediaType{
								"application/json": {
									Schema: &SchemaSpec{
										Type: "object",
										Properties: map[string]*SchemaSpec{
											"error": {Type: "string"},
										},
										Required: []string{"error"},
									},
								},
							},
						},
					},
				},
				"post": {
					Responses: map[string]*ResponseSpec{
						"201": {
							Description: "Task created successfully",
							Content: map[string]*MediaType{
								"application/json": {
									Schema: &SchemaSpec{
										Type: "object",
										Properties: map[string]*SchemaSpec{
											"id":             {Type: "string"},
											"name":           {Type: "string"},
											"status":         {Type: "string"},
											"script_content": {Type: "string"},
											"script_type":    {Type: "string"},
											"created_at":     {Type: "string"},
										},
										Required: []string{"id", "name", "status", "script_content", "script_type", "created_at"},
									},
								},
							},
						},
						"401": {
							Description: "Unauthorized access",
							Content: map[string]*MediaType{
								"application/json": {
									Schema: &SchemaSpec{
										Type: "object",
										Properties: map[string]*SchemaSpec{
											"error": {Type: "string"},
										},
										Required: []string{"error"},
									},
								},
							},
						},
					},
				},
			},
			"/health": {
				"get": {
					Responses: map[string]*ResponseSpec{
						"200": {
							Description: "Health check response",
							Content: map[string]*MediaType{
								"application/json": {
									Schema: &SchemaSpec{
										Type: "object",
										Properties: map[string]*SchemaSpec{
											"status":    {Type: "string"},
											"timestamp": {Type: "string"},
										},
										Required: []string{"status", "timestamp"},
									},
								},
							},
						},
					},
				},
			},
			"/ready": {
				"get": {
					Responses: map[string]*ResponseSpec{
						"200": {
							Description: "Readiness check response",
							Content: map[string]*MediaType{
								"application/json": {
									Schema: &SchemaSpec{
										Type: "object",
										Properties: map[string]*SchemaSpec{
											"status":    {Type: "string"},
											"timestamp": {Type: "string"},
										},
										Required: []string{"status", "timestamp"},
									},
								},
							},
						},
					},
				},
			},
			"/api/v1/tasks/{id}": {
				"get": {
					Responses: map[string]*ResponseSpec{
						"200": {
							Description: "Task details",
							Content: map[string]*MediaType{
								"application/json": {
									Schema: &SchemaSpec{
										Type: "object",
										Properties: map[string]*SchemaSpec{
											"id":             {Type: "string"},
											"name":           {Type: "string"},
											"status":         {Type: "string"},
											"script_content": {Type: "string"},
											"script_type":    {Type: "string"},
											"created_at":     {Type: "string"},
										},
										Required: []string{"id", "name", "status", "script_content", "script_type", "created_at"},
									},
								},
							},
						},
						"401": {
							Description: "Unauthorized access",
							Content: map[string]*MediaType{
								"application/json": {
									Schema: &SchemaSpec{
										Type: "object",
										Properties: map[string]*SchemaSpec{
											"error": {Type: "string"},
										},
										Required: []string{"error"},
									},
								},
							},
						},
						"404": {
							Description: "Task not found",
							Content: map[string]*MediaType{
								"application/json": {
									Schema: &SchemaSpec{
										Type: "object",
										Properties: map[string]*SchemaSpec{
											"error": {Type: "string"},
										},
										Required: []string{"error"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// ValidateResponse validates an HTTP response against the OpenAPI specification
func (v *OpenAPIValidator) ValidateResponse(t *testing.T, method, path string, response *http.Response, body []byte) {
	t.Helper()

	// Normalize path to remove query parameters and path parameters
	normalizedPath := v.normalizePath(path)

	// Get operation spec
	pathSpec, exists := v.spec.Paths[normalizedPath]
	if !exists {
		t.Logf("Warning: No OpenAPI spec found for path %s", normalizedPath)
		return
	}

	operationSpec, exists := pathSpec[strings.ToLower(method)]
	if !exists {
		t.Logf("Warning: No OpenAPI spec found for %s %s", method, normalizedPath)
		return
	}

	// Get response spec
	statusCode := fmt.Sprintf("%d", response.StatusCode)
	responseSpec, exists := operationSpec.Responses[statusCode]
	if !exists {
		// Try to find a default response spec
		if defaultSpec, hasDefault := operationSpec.Responses["default"]; hasDefault {
			responseSpec = defaultSpec
		} else {
			t.Errorf("No OpenAPI response spec found for %s %s with status %d", method, normalizedPath, response.StatusCode)
			return
		}
	}

	// Validate content type
	contentType := response.Header.Get("Content-Type")
	if contentType != "" {
		// Remove charset and other parameters
		mainContentType := strings.Split(contentType, ";")[0]

		if responseSpec.Content != nil {
			if _, exists := responseSpec.Content[mainContentType]; !exists {
				t.Errorf("Unexpected content type %s for %s %s (status %d)", mainContentType, method, normalizedPath, response.StatusCode)
			}
		}
	}

	// Validate JSON response body structure
	if strings.Contains(contentType, "application/json") && len(body) > 0 {
		v.validateJSONResponse(t, method, normalizedPath, statusCode, responseSpec, body)
	}

	// Validate headers
	v.validateHeaders(t, method, normalizedPath, statusCode, responseSpec, response.Header)
}

// normalizePath removes query parameters and normalizes path parameters
func (v *OpenAPIValidator) normalizePath(path string) string {
	// Remove query parameters
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	// Normalize path parameters (e.g., /api/v1/tasks/123 -> /api/v1/tasks/{id})
	// This is a simplified implementation - in production, you'd need more sophisticated path matching
	parts := strings.Split(path, "/")
	for i, part := range parts {
		// If part looks like a UUID or ID, replace with parameter placeholder
		if len(part) > 20 && (strings.Contains(part, "-") || isNumeric(part)) {
			parts[i] = "{id}"
		}
	}

	return strings.Join(parts, "/")
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

// validateJSONResponse validates the JSON response body against the schema
func (v *OpenAPIValidator) validateJSONResponse(t *testing.T, method, path, statusCode string, responseSpec *ResponseSpec, body []byte) {
	t.Helper()

	var responseBody interface{}
	err := json.Unmarshal(body, &responseBody)
	require.NoError(t, err, "Response body should be valid JSON for %s %s (status %s)", method, path, statusCode)

	// Get the JSON schema for this content type
	if responseSpec.Content == nil {
		return
	}

	jsonContent, exists := responseSpec.Content["application/json"]
	if !exists {
		return
	}

	if jsonContent.Schema == nil {
		return
	}

	// Validate against schema
	v.validateAgainstSchema(t, method, path, statusCode, jsonContent.Schema, responseBody, "root")
}

// validateAgainstSchema validates a value against a JSON schema
func (v *OpenAPIValidator) validateAgainstSchema(t *testing.T, method, path, statusCode string, schema *SchemaSpec, value interface{}, fieldPath string) {
	t.Helper()

	switch schema.Type {
	case "object":
		obj, ok := value.(map[string]interface{})
		if !ok {
			t.Errorf("Expected object at %s for %s %s (status %s), got %T", fieldPath, method, path, statusCode, value)
			return
		}

		// Check required fields
		for _, required := range schema.Required {
			if _, exists := obj[required]; !exists {
				t.Errorf("Missing required field '%s' at %s for %s %s (status %s)", required, fieldPath, method, path, statusCode)
			}
		}

		// Validate properties
		if schema.Properties != nil {
			for propName, propSchema := range schema.Properties {
				if propValue, exists := obj[propName]; exists {
					newPath := fmt.Sprintf("%s.%s", fieldPath, propName)
					v.validateAgainstSchema(t, method, path, statusCode, propSchema, propValue, newPath)
				}
			}
		}

	case "array":
		arr, ok := value.([]interface{})
		if !ok {
			t.Errorf("Expected array at %s for %s %s (status %s), got %T", fieldPath, method, path, statusCode, value)
			return
		}

		// Validate array items
		if schema.Items != nil {
			for i, item := range arr {
				newPath := fmt.Sprintf("%s[%d]", fieldPath, i)
				v.validateAgainstSchema(t, method, path, statusCode, schema.Items, item, newPath)
			}
		}

	case "string":
		if _, ok := value.(string); !ok {
			t.Errorf("Expected string at %s for %s %s (status %s), got %T", fieldPath, method, path, statusCode, value)
		}

	case "integer":
		// JSON numbers can be float64, so we need to check if it's a whole number
		if num, ok := value.(float64); ok {
			if num != float64(int64(num)) {
				t.Errorf("Expected integer at %s for %s %s (status %s), got float %v", fieldPath, method, path, statusCode, num)
			}
		} else {
			t.Errorf("Expected integer at %s for %s %s (status %s), got %T", fieldPath, method, path, statusCode, value)
		}

	case "number":
		if _, ok := value.(float64); !ok {
			t.Errorf("Expected number at %s for %s %s (status %s), got %T", fieldPath, method, path, statusCode, value)
		}

	case "boolean":
		if _, ok := value.(bool); !ok {
			t.Errorf("Expected boolean at %s for %s %s (status %s), got %T", fieldPath, method, path, statusCode, value)
		}
	}
}

// validateHeaders validates response headers against the specification
func (v *OpenAPIValidator) validateHeaders(t *testing.T, method, path, statusCode string, responseSpec *ResponseSpec, headers http.Header) {
	t.Helper()

	if responseSpec.Headers == nil {
		return
	}

	for headerName, headerSpec := range responseSpec.Headers {
		headerValue := headers.Get(headerName)
		if headerValue == "" {
			t.Errorf("Missing expected header '%s' for %s %s (status %s)", headerName, method, path, statusCode)
			continue
		}

		// Validate header value type if schema is specified
		if headerSpec.Schema != nil {
			// Simple validation for string headers
			if headerSpec.Schema.Type == "string" && headerValue == "" {
				t.Errorf("Header '%s' should not be empty for %s %s (status %s)", headerName, method, path, statusCode)
			}
		}
	}
}

// ValidateCommonHeaders validates common HTTP headers that should be present
func (v *OpenAPIValidator) ValidateCommonHeaders(t *testing.T, response *http.Response) {
	t.Helper()

	// Validate Content-Type for JSON responses
	contentType := response.Header.Get("Content-Type")
	if response.StatusCode < 300 && contentType != "" {
		assert.Contains(t, contentType, "application/json", "JSON responses should have application/json content type")
	}

	// Validate security headers for sensitive endpoints
	if response.Request != nil && response.Request.URL != nil && strings.Contains(response.Request.URL.Path, "/auth/") {
		// These are important security headers that should be present
		assert.NotEmpty(t, response.Header.Get("X-Content-Type-Options"), "X-Content-Type-Options header should be present for auth endpoints")
	}
}

// HTTPResponseValidator provides a fluent interface for HTTP response validation
type HTTPResponseValidator struct {
	t         *testing.T
	response  *http.Response
	body      []byte
	validator *OpenAPIValidator
}

// NewHTTPResponseValidator creates a new HTTP response validator
func NewHTTPResponseValidator(t *testing.T, response *http.Response) *HTTPResponseValidator {
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	return &HTTPResponseValidator{
		t:         t,
		response:  response,
		body:      body,
		validator: NewOpenAPIValidator(),
	}
}

// ExpectStatus validates the HTTP status code
func (v *HTTPResponseValidator) ExpectStatus(expectedStatus int) *HTTPResponseValidator {
	assert.Equal(v.t, expectedStatus, v.response.StatusCode, "Unexpected status code")
	return v
}

// ExpectContentType validates the Content-Type header
func (v *HTTPResponseValidator) ExpectContentType(expectedContentType string) *HTTPResponseValidator {
	contentType := v.response.Header.Get("Content-Type")
	assert.Contains(v.t, contentType, expectedContentType, "Unexpected content type")
	return v
}

// ExpectValidJSON validates that the response body is valid JSON
func (v *HTTPResponseValidator) ExpectValidJSON() *HTTPResponseValidator {
	var obj interface{}
	err := json.Unmarshal(v.body, &obj)
	assert.NoError(v.t, err, "Response body should be valid JSON")
	return v
}

// ExpectOpenAPICompliance validates the response against OpenAPI specification
func (v *HTTPResponseValidator) ExpectOpenAPICompliance(method, path string) *HTTPResponseValidator {
	v.validator.ValidateResponse(v.t, method, path, v.response, v.body)
	return v
}

// ExpectCommonHeaders validates common HTTP headers
func (v *HTTPResponseValidator) ExpectCommonHeaders() *HTTPResponseValidator {
	v.validator.ValidateCommonHeaders(v.t, v.response)
	return v
}

// GetBody returns the response body for further custom validation
func (v *HTTPResponseValidator) GetBody() []byte {
	return v.body
}

// UnmarshalResponse unmarshals the response body into the provided struct
func (v *HTTPResponseValidator) UnmarshalResponse(dest interface{}) *HTTPResponseValidator {
	err := json.Unmarshal(v.body, dest)
	require.NoError(v.t, err, "Failed to unmarshal response body")
	return v
}
