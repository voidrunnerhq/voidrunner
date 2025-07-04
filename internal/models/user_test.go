package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid email",
			email:   "test@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with subdomain",
			email:   "user@subdomain.example.com",
			wantErr: false,
		},
		{
			name:    "empty email",
			email:   "",
			wantErr: true,
			errMsg:  "email is required",
		},
		{
			name:    "email without @",
			email:   "invalidemail",
			wantErr: true,
			errMsg:  "invalid email format",
		},
		{
			name:    "email without domain",
			email:   "test@",
			wantErr: true,
			errMsg:  "invalid email format",
		},
		{
			name:    "email without local part",
			email:   "@example.com",
			wantErr: true,
			errMsg:  "invalid email format",
		},
		{
			name:    "email too long",
			email:   string(make([]byte, 250)) + "@example.com",
			wantErr: true,
			errMsg:  "email is too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid password",
			password: "Password123",
			wantErr:  false,
		},
		{
			name:     "valid complex password",
			password: "MyStr0ngP@ssw0rd!",
			wantErr:  false,
		},
		{
			name:     "too short password",
			password: "Pass1",
			wantErr:  true,
			errMsg:   "at least 8 characters",
		},
		{
			name:     "too long password",
			password: string(make([]byte, 130)),
			wantErr:  true,
			errMsg:   "too long",
		},
		{
			name:     "no uppercase letter",
			password: "password123",
			wantErr:  true,
			errMsg:   "uppercase letter",
		},
		{
			name:     "no lowercase letter",
			password: "PASSWORD123",
			wantErr:  true,
			errMsg:   "lowercase letter",
		},
		{
			name:     "no digit",
			password: "Password",
			wantErr:  true,
			errMsg:   "digit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUser_ToResponse(t *testing.T) {
	user := &User{
		BaseModel: BaseModel{
			ID: NewID(),
		},
		Email: "test@example.com",
	}

	response := user.ToResponse()

	assert.Equal(t, user.ID, response.ID)
	assert.Equal(t, user.Email, response.Email)
	assert.NotEmpty(t, response.CreatedAt)
	assert.NotEmpty(t, response.UpdatedAt)
}