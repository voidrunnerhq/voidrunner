package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
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