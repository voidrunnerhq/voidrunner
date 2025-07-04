package models

import (
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