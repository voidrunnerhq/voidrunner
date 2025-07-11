package models

import (
	"database/sql/driver"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewID(t *testing.T) {
	id1 := NewID()
	id2 := NewID()

	// IDs should be valid UUIDs
	assert.NotEqual(t, uuid.Nil, id1)
	assert.NotEqual(t, uuid.Nil, id2)

	// IDs should be different
	assert.NotEqual(t, id1, id2)
}

func TestValidateID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid UUID string",
			input:   "550e8400-e29b-41d4-a716-446655440000",
			wantErr: false,
		},
		{
			name:    "valid UUID string with different format",
			input:   NewID().String(),
			wantErr: false,
		},
		{
			name:    "invalid UUID string",
			input:   "invalid-uuid",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "UUID string with wrong length",
			input:   "550e8400-e29b-41d4-a716",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ValidateID(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, uuid.Nil, id)
			} else {
				assert.NoError(t, err)
				assert.NotEqual(t, uuid.Nil, id)
			}
		})
	}
}

func TestJSONB_Scan(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expected    JSONB
		expectError bool
	}{
		{
			name:     "scan nil value",
			input:    nil,
			expected: JSONB{},
		},
		{
			name:     "scan valid JSON bytes",
			input:    []byte(`{"key": "value", "number": 42}`),
			expected: JSONB{"key": "value", "number": float64(42)},
		},
		{
			name:     "scan valid JSON string",
			input:    `{"name": "test", "active": true}`,
			expected: JSONB{"name": "test", "active": true},
		},
		{
			name:     "scan empty JSON object",
			input:    []byte(`{}`),
			expected: JSONB{},
		},
		{
			name:     "scan complex nested JSON",
			input:    []byte(`{"user": {"id": 1, "tags": ["admin", "user"]}, "count": 10}`),
			expected: JSONB{"user": map[string]interface{}{"id": float64(1), "tags": []interface{}{"admin", "user"}}, "count": float64(10)},
		},
		{
			name:        "scan invalid JSON bytes",
			input:       []byte(`{"invalid": json}`),
			expectError: true,
		},
		{
			name:        "scan invalid JSON string",
			input:       `{"malformed": "json"`,
			expectError: true,
		},
		{
			name:        "scan unsupported type",
			input:       123,
			expectError: true,
		},
		{
			name:        "scan boolean type",
			input:       true,
			expectError: true,
		},
		{
			name:        "scan struct type",
			input:       struct{ Field string }{Field: "value"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var j JSONB
			err := j.Scan(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, j)
			}
		})
	}
}

func TestJSONB_Value(t *testing.T) {
	tests := []struct {
		name        string
		input       JSONB
		expected    driver.Value
		expectError bool
	}{
		{
			name:     "value from nil JSONB",
			input:    nil,
			expected: nil,
		},
		{
			name:     "value from empty JSONB",
			input:    JSONB{},
			expected: []byte(`{}`),
		},
		{
			name:     "value from simple JSONB",
			input:    JSONB{"key": "value"},
			expected: []byte(`{"key":"value"}`),
		},
		{
			name:     "value from complex JSONB",
			input:    JSONB{"user": map[string]interface{}{"id": 1, "name": "test"}, "active": true},
			expected: []byte(`{"active":true,"user":{"id":1,"name":"test"}}`),
		},
		{
			name:     "value from JSONB with different types",
			input:    JSONB{"string": "text", "number": 42, "boolean": true, "null": nil},
			expected: []byte(`{"boolean":true,"null":null,"number":42,"string":"text"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.input.Value()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expected == nil {
					assert.Nil(t, value)
				} else {
					// For JSON, we need to check the content since order may vary
					expectedJSON := tt.expected.([]byte)
					actualJSON := value.([]byte)

					var expectedMap, actualMap map[string]interface{}
					require.NoError(t, json.Unmarshal(expectedJSON, &expectedMap))
					require.NoError(t, json.Unmarshal(actualJSON, &actualMap))

					assert.Equal(t, expectedMap, actualMap)
				}
			}
		})
	}
}

func TestJSONB_Value_ErrorScenarios(t *testing.T) {
	t.Run("value with un-marshalable content", func(t *testing.T) {
		// Create a JSONB with a function value that can't be marshaled
		j := JSONB{
			"function": func() {},
		}

		value, err := j.Value()
		assert.Error(t, err)
		assert.Nil(t, value)
		assert.Contains(t, err.Error(), "json: unsupported type")
	})

	t.Run("value with circular reference", func(t *testing.T) {
		// Create a circular reference that can't be marshaled
		circular := make(map[string]interface{})
		circular["self"] = circular

		j := JSONB{
			"circular": circular,
		}

		value, err := j.Value()
		assert.Error(t, err)
		assert.Nil(t, value)
		assert.Contains(t, err.Error(), "json: unsupported value: encountered a cycle")
	})

	t.Run("value with channel type", func(t *testing.T) {
		ch := make(chan int)
		defer close(ch)

		j := JSONB{
			"channel": ch,
		}

		value, err := j.Value()
		assert.Error(t, err)
		assert.Nil(t, value)
		assert.Contains(t, err.Error(), "json: unsupported type")
	})
}

func TestJSONB_MarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       JSONB
		expected    string
		expectError bool
	}{
		{
			name:     "marshal empty JSONB",
			input:    JSONB{},
			expected: `{}`,
		},
		{
			name:     "marshal simple JSONB",
			input:    JSONB{"key": "value"},
			expected: `{"key":"value"}`,
		},
		{
			name:     "marshal complex JSONB",
			input:    JSONB{"user": map[string]interface{}{"id": 1, "name": "test"}, "active": true},
			expected: `{"active":true,"user":{"id":1,"name":"test"}}`,
		},
		{
			name:     "marshal JSONB with nil values",
			input:    JSONB{"nullValue": nil, "emptyString": ""},
			expected: `{"emptyString":"","nullValue":null}`,
		},
		{
			name:     "marshal JSONB with arrays",
			input:    JSONB{"tags": []interface{}{"admin", "user"}, "numbers": []interface{}{1, 2, 3}},
			expected: `{"numbers":[1,2,3],"tags":["admin","user"]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.input.MarshalJSON()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Parse both expected and actual JSON to compare content
				var expectedMap, actualMap map[string]interface{}
				require.NoError(t, json.Unmarshal([]byte(tt.expected), &expectedMap))
				require.NoError(t, json.Unmarshal(result, &actualMap))

				assert.Equal(t, expectedMap, actualMap)
			}
		})
	}
}

func TestJSONB_MarshalJSON_ErrorScenarios(t *testing.T) {
	t.Run("marshal with un-marshalable content", func(t *testing.T) {
		j := JSONB{
			"function": func() {},
		}

		result, err := j.MarshalJSON()
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "json: unsupported type")
	})

	t.Run("marshal with channel", func(t *testing.T) {
		ch := make(chan string)
		defer close(ch)

		j := JSONB{
			"channel": ch,
		}

		result, err := j.MarshalJSON()
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "json: unsupported type")
	})
}

func TestJSONB_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    JSONB
		expectError bool
	}{
		{
			name:     "unmarshal empty object",
			input:    `{}`,
			expected: JSONB{},
		},
		{
			name:     "unmarshal simple object",
			input:    `{"key": "value"}`,
			expected: JSONB{"key": "value"},
		},
		{
			name:     "unmarshal complex object",
			input:    `{"user": {"id": 1, "name": "test"}, "active": true}`,
			expected: JSONB{"user": map[string]interface{}{"id": float64(1), "name": "test"}, "active": true},
		},
		{
			name:     "unmarshal object with null values",
			input:    `{"nullValue": null, "emptyString": ""}`,
			expected: JSONB{"nullValue": nil, "emptyString": ""},
		},
		{
			name:     "unmarshal object with arrays",
			input:    `{"tags": ["admin", "user"], "numbers": [1, 2, 3]}`,
			expected: JSONB{"tags": []interface{}{"admin", "user"}, "numbers": []interface{}{float64(1), float64(2), float64(3)}},
		},
		{
			name:        "unmarshal invalid JSON",
			input:       `{"invalid": json}`,
			expectError: true,
		},
		{
			name:        "unmarshal malformed JSON",
			input:       `{"key": "value"`,
			expectError: true,
		},
		{
			name:        "unmarshal non-object JSON",
			input:       `["array", "not", "object"]`,
			expectError: true,
		},
		{
			name:        "unmarshal primitive JSON",
			input:       `"string"`,
			expectError: true,
		},
		{
			name:        "unmarshal empty string",
			input:       ``,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var j JSONB
			err := j.UnmarshalJSON([]byte(tt.input))

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, j)
			}
		})
	}
}

func TestJSONB_RoundTrip(t *testing.T) {
	t.Run("scan and value round trip", func(t *testing.T) {
		original := JSONB{
			"user": map[string]interface{}{
				"id":   float64(123),
				"name": "test user",
				"settings": map[string]interface{}{
					"theme":         "dark",
					"notifications": true,
				},
			},
			"tags":  []interface{}{"admin", "power-user"},
			"count": float64(42),
		}

		// Convert to database value
		value, err := original.Value()
		require.NoError(t, err)

		// Scan back from database value
		var scanned JSONB
		err = scanned.Scan(value)
		require.NoError(t, err)

		assert.Equal(t, original, scanned)
	})

	t.Run("marshal and unmarshal round trip", func(t *testing.T) {
		original := JSONB{
			"metadata": map[string]interface{}{
				"version":  float64(1),
				"features": []interface{}{"feature1", "feature2"},
			},
			"enabled": true,
		}

		// Marshal to JSON
		jsonData, err := original.MarshalJSON()
		require.NoError(t, err)

		// Unmarshal back from JSON
		var unmarshaled JSONB
		err = unmarshaled.UnmarshalJSON(jsonData)
		require.NoError(t, err)

		assert.Equal(t, original, unmarshaled)
	})
}
