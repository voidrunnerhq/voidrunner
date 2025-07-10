package models

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			password: "Password123!",
			wantErr:  false,
		},
		{
			name:     "valid complex password",
			password: "MyStr0ngP@ssw0rd!",
			wantErr:  false,
		},
		{
			name:     "too short password",
			password: "Pass1!",
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
			password: "password123!",
			wantErr:  true,
			errMsg:   "uppercase letter",
		},
		{
			name:     "no lowercase letter",
			password: "PASSWORD123!",
			wantErr:  true,
			errMsg:   "lowercase letter",
		},
		{
			name:     "no digit",
			password: "Password!",
			wantErr:  true,
			errMsg:   "digit",
		},
		{
			name:     "no special character",
			password: "Password123",
			wantErr:  true,
			errMsg:   "special character",
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
			ID:        NewID(),
			CreatedAt: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC),
		},
		Email: "test@example.com",
		Name:  "Test User",
	}

	response := user.ToResponse()

	assert.Equal(t, user.ID, response.ID)
	assert.Equal(t, user.Email, response.Email)
	assert.Equal(t, user.Name, response.Name)
	assert.Equal(t, "2023-01-01T12:00:00Z", response.CreatedAt)
	assert.Equal(t, "2023-01-02T12:00:00Z", response.UpdatedAt)
}

func TestUser_ToJWTClaims(t *testing.T) {
	userID := uuid.New()
	now := time.Now()
	expiresAt := now.Add(time.Hour)

	user := &User{
		BaseModel: BaseModel{
			ID:        userID,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Email: "test@example.com",
		Name:  "Test User",
	}

	claims := user.ToJWTClaims("access", "voidrunner", "voidrunner-api", expiresAt)

	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, "test@example.com", claims.Email)
	assert.Equal(t, "access", claims.Type)
	assert.Equal(t, "voidrunner", claims.Issuer)
	assert.Equal(t, userID.String(), claims.Subject)
	assert.Equal(t, jwt.ClaimStrings{"voidrunner-api"}, claims.Audience)
	assert.Equal(t, jwt.NewNumericDate(expiresAt), claims.ExpiresAt)
	assert.NotNil(t, claims.NotBefore)
	assert.NotNil(t, claims.IssuedAt)
}

func TestUser_ToJWTClaims_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name      string
		user      *User
		tokenType string
		issuer    string
		audience  string
		expiresAt time.Time
	}{
		{
			name: "zero UUID user ID",
			user: &User{
				BaseModel: BaseModel{
					ID:        uuid.UUID{}, // Zero UUID
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Email: "test@example.com",
				Name:  "Test User",
			},
			tokenType: "access",
			issuer:    "voidrunner",
			audience:  "voidrunner-api",
			expiresAt: time.Now().Add(time.Hour),
		},
		{
			name: "empty email",
			user: &User{
				BaseModel: BaseModel{
					ID:        uuid.New(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Email: "",
				Name:  "Test User",
			},
			tokenType: "access",
			issuer:    "voidrunner",
			audience:  "voidrunner-api",
			expiresAt: time.Now().Add(time.Hour),
		},
		{
			name: "empty token type",
			user: &User{
				BaseModel: BaseModel{
					ID:        uuid.New(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Email: "test@example.com",
				Name:  "Test User",
			},
			tokenType: "",
			issuer:    "voidrunner",
			audience:  "voidrunner-api",
			expiresAt: time.Now().Add(time.Hour),
		},
		{
			name: "empty issuer",
			user: &User{
				BaseModel: BaseModel{
					ID:        uuid.New(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Email: "test@example.com",
				Name:  "Test User",
			},
			tokenType: "access",
			issuer:    "",
			audience:  "voidrunner-api",
			expiresAt: time.Now().Add(time.Hour),
		},
		{
			name: "empty audience",
			user: &User{
				BaseModel: BaseModel{
					ID:        uuid.New(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Email: "test@example.com",
				Name:  "Test User",
			},
			tokenType: "access",
			issuer:    "voidrunner",
			audience:  "",
			expiresAt: time.Now().Add(time.Hour),
		},
		{
			name: "past expiration time",
			user: &User{
				BaseModel: BaseModel{
					ID:        uuid.New(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Email: "test@example.com",
				Name:  "Test User",
			},
			tokenType: "access",
			issuer:    "voidrunner",
			audience:  "voidrunner-api",
			expiresAt: time.Now().Add(-time.Hour), // Past expiration
		},
		{
			name: "zero expiration time",
			user: &User{
				BaseModel: BaseModel{
					ID:        uuid.New(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Email: "test@example.com",
				Name:  "Test User",
			},
			tokenType: "access",
			issuer:    "voidrunner",
			audience:  "voidrunner-api",
			expiresAt: time.Time{}, // Zero time
		},
		{
			name: "extremely long token type",
			user: &User{
				BaseModel: BaseModel{
					ID:        uuid.New(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Email: "test@example.com",
				Name:  "Test User",
			},
			tokenType: "access_token_with_extremely_long_type_name_that_might_cause_issues_in_jwt_processing_or_validation_systems",
			issuer:    "voidrunner",
			audience:  "voidrunner-api",
			expiresAt: time.Now().Add(time.Hour),
		},
		{
			name: "extremely long issuer",
			user: &User{
				BaseModel: BaseModel{
					ID:        uuid.New(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Email: "test@example.com",
				Name:  "Test User",
			},
			tokenType: "access",
			issuer:    "voidrunner_with_extremely_long_issuer_name_that_might_cause_problems_in_jwt_token_generation_or_validation_processes",
			audience:  "voidrunner-api",
			expiresAt: time.Now().Add(time.Hour),
		},
		{
			name: "extremely long audience",
			user: &User{
				BaseModel: BaseModel{
					ID:        uuid.New(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Email: "test@example.com",
				Name:  "Test User",
			},
			tokenType: "access",
			issuer:    "voidrunner",
			audience:  "voidrunner-api-with-extremely-long-audience-name-that-might-cause-jwt-token-issues",
			expiresAt: time.Now().Add(time.Hour),
		},
		{
			name: "invalid email format in claims",
			user: &User{
				BaseModel: BaseModel{
					ID:        uuid.New(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Email: "not-an-email",
				Name:  "Test User",
			},
			tokenType: "access",
			issuer:    "voidrunner",
			audience:  "voidrunner-api",
			expiresAt: time.Now().Add(time.Hour),
		},
		{
			name: "special characters in token type",
			user: &User{
				BaseModel: BaseModel{
					ID:        uuid.New(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Email: "test@example.com",
				Name:  "Test User",
			},
			tokenType: "access-token!@#$%^&*()",
			issuer:    "voidrunner",
			audience:  "voidrunner-api",
			expiresAt: time.Now().Add(time.Hour),
		},
		{
			name: "unicode characters in email",
			user: &User{
				BaseModel: BaseModel{
					ID:        uuid.New(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Email: "test@例え.テスト",
				Name:  "Test User",
			},
			tokenType: "access",
			issuer:    "voidrunner",
			audience:  "voidrunner-api",
			expiresAt: time.Now().Add(time.Hour),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that ToJWTClaims doesn't panic and returns valid claims structure
			claims := tt.user.ToJWTClaims(tt.tokenType, tt.issuer, tt.audience, tt.expiresAt)

			// Verify basic structure is maintained
			assert.Equal(t, tt.user.ID, claims.UserID)
			assert.Equal(t, tt.user.Email, claims.Email)
			assert.Equal(t, tt.tokenType, claims.Type)
			assert.Equal(t, tt.issuer, claims.Issuer)
			assert.Equal(t, tt.user.ID.String(), claims.Subject)
			assert.Equal(t, jwt.ClaimStrings{tt.audience}, claims.Audience)
			assert.Equal(t, jwt.NewNumericDate(tt.expiresAt), claims.ExpiresAt)
			assert.NotNil(t, claims.NotBefore)
			assert.NotNil(t, claims.IssuedAt)

			// Verify time relationships
			assert.True(t, claims.IssuedAt.Before(claims.NotBefore.Time) || claims.IssuedAt.Equal(claims.NotBefore.Time))

			// Test that claims can be serialized (important for JWT token generation)
			require.NotPanics(t, func() {
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				_, err := token.SignedString([]byte("test-secret"))
				assert.NoError(t, err)
			})
		})
	}
}

func TestUser_ToJWTClaims_EdgeCases(t *testing.T) {
	t.Run("nil user pointer", func(t *testing.T) {
		defer func() {
			r := recover()
			assert.NotNil(t, r, "should panic when called on nil user")
		}()

		var user *User
		user.ToJWTClaims("access", "voidrunner", "voidrunner-api", time.Now().Add(time.Hour))
	})

	t.Run("very distant future expiration", func(t *testing.T) {
		user := &User{
			BaseModel: BaseModel{
				ID:        uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			Email: "test@example.com",
			Name:  "Test User",
		}

		// Test with expiration far in the future
		farFuture := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
		claims := user.ToJWTClaims("access", "voidrunner", "voidrunner-api", farFuture)

		assert.Equal(t, jwt.NewNumericDate(farFuture), claims.ExpiresAt)
		assert.True(t, claims.ExpiresAt.After(time.Now()))
	})

	t.Run("very distant past expiration", func(t *testing.T) {
		user := &User{
			BaseModel: BaseModel{
				ID:        uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			Email: "test@example.com",
			Name:  "Test User",
		}

		// Test with expiration far in the past
		farPast := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
		claims := user.ToJWTClaims("access", "voidrunner", "voidrunner-api", farPast)

		assert.Equal(t, jwt.NewNumericDate(farPast), claims.ExpiresAt)
		assert.True(t, claims.ExpiresAt.Before(time.Now()))
	})

	t.Run("concurrent access", func(t *testing.T) {
		user := &User{
			BaseModel: BaseModel{
				ID:        uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			Email: "test@example.com",
			Name:  "Test User",
		}

		// Test concurrent access to ToJWTClaims
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(i int) {
				defer func() {
					done <- true
				}()
				expiresAt := time.Now().Add(time.Duration(i) * time.Hour)
				claims := user.ToJWTClaims("access", "voidrunner", "voidrunner-api", expiresAt)
				assert.Equal(t, user.ID, claims.UserID)
				assert.Equal(t, user.Email, claims.Email)
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("maximum length fields", func(t *testing.T) {
		// Create strings at or near maximum practical lengths
		maxEmail := "a" + "@" + "b" + ".com"      // Keep email reasonable
		maxTokenType := string(make([]byte, 100)) // Large token type
		maxIssuer := string(make([]byte, 200))    // Large issuer
		maxAudience := string(make([]byte, 300))  // Large audience

		for i := range maxTokenType {
			maxTokenType = maxTokenType[:i] + "a" + maxTokenType[i+1:]
		}
		for i := range maxIssuer {
			maxIssuer = maxIssuer[:i] + "b" + maxIssuer[i+1:]
		}
		for i := range maxAudience {
			maxAudience = maxAudience[:i] + "c" + maxAudience[i+1:]
		}

		user := &User{
			BaseModel: BaseModel{
				ID:        uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			Email: maxEmail,
			Name:  "Test User",
		}

		claims := user.ToJWTClaims(maxTokenType, maxIssuer, maxAudience, time.Now().Add(time.Hour))

		assert.Equal(t, user.ID, claims.UserID)
		assert.Equal(t, maxEmail, claims.Email)
		assert.Equal(t, maxTokenType, claims.Type)
		assert.Equal(t, maxIssuer, claims.Issuer)
		assert.Equal(t, jwt.ClaimStrings{maxAudience}, claims.Audience)
	})
}
