package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// BaseModel contains common fields for all models
type BaseModel struct {
	ID        uuid.UUID `json:"id" db:"id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// NewID generates a new UUID
func NewID() uuid.UUID {
	return uuid.New()
}

// ValidateID checks if an ID is valid
func ValidateID(id string) (uuid.UUID, error) {
	return uuid.Parse(id)
}

// ErrorResponse represents a general error response
type ErrorResponse struct {
	Error            string            `json:"error"`
	Details          string            `json:"details,omitempty"`
	ValidationErrors []ValidationError `json:"validation_errors,omitempty"`
}

// ValidationError represents a field validation error
type ValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value"`
	Tag     string `json:"tag"`
	Message string `json:"message"`
}

// JSONB represents a JSONB field that can be scanned from database and marshaled to JSON
type JSONB map[string]interface{}

// Scan implements the sql.Scanner interface for database scanning
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(map[string]interface{})
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into JSONB", value)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return fmt.Errorf("cannot unmarshal JSON into JSONB: %w", err)
	}

	*j = JSONB(result)
	return nil
}

// Value implements the driver.Valuer interface for database storage
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(map[string]interface{}(j))
}

// MarshalJSON implements the json.Marshaler interface
func (j JSONB) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}(j))
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (j *JSONB) UnmarshalJSON(data []byte) error {
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	*j = JSONB(result)
	return nil
}
