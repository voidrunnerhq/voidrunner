package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

func TestJWTService(t *testing.T) {
	// Setup test config
	jwtConfig := &config.JWTConfig{
		SecretKey:            "test-secret-key-for-testing-only",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: 7 * 24 * time.Hour,
		Issuer:               "voidrunner-test",
		Audience:             "voidrunner-api-test",
	}

	service := NewJWTService(jwtConfig)

	// Create test user
	user := &models.User{
		BaseModel: models.BaseModel{
			ID: uuid.New(),
		},
		Email: "test@example.com",
	}

	t.Run("Generate Token Pair", func(t *testing.T) {
		tokenPair, err := service.GenerateTokenPair(user)
		require.NoError(t, err)
		require.NotNil(t, tokenPair)

		assert.NotEmpty(t, tokenPair.AccessToken)
		assert.NotEmpty(t, tokenPair.RefreshToken)
		assert.Equal(t, int64(900), tokenPair.ExpiresIn) // 15 minutes
	})

	t.Run("Validate Access Token", func(t *testing.T) {
		tokenPair, err := service.GenerateTokenPair(user)
		require.NoError(t, err)

		claims, err := service.ValidateToken(tokenPair.AccessToken)
		require.NoError(t, err)
		require.NotNil(t, claims)

		assert.Equal(t, user.ID, claims.UserID)
		assert.Equal(t, user.Email, claims.Email)
		assert.Equal(t, "access", claims.Type)
		assert.Equal(t, jwtConfig.Issuer, claims.Issuer)
	})

	t.Run("Validate Refresh Token", func(t *testing.T) {
		tokenPair, err := service.GenerateTokenPair(user)
		require.NoError(t, err)

		claims, err := service.ValidateToken(tokenPair.RefreshToken)
		require.NoError(t, err)
		require.NotNil(t, claims)

		assert.Equal(t, user.ID, claims.UserID)
		assert.Equal(t, user.Email, claims.Email)
		assert.Equal(t, "refresh", claims.Type)
		assert.Equal(t, jwtConfig.Issuer, claims.Issuer)
	})

	t.Run("Refresh Token", func(t *testing.T) {
		tokenPair, err := service.GenerateTokenPair(user)
		require.NoError(t, err)

		// Add a small delay to ensure different issued times
		time.Sleep(time.Millisecond * 10)

		newTokenPair, err := service.RefreshToken(tokenPair.RefreshToken)
		require.NoError(t, err)
		require.NotNil(t, newTokenPair)

		assert.NotEmpty(t, newTokenPair.AccessToken)
		assert.NotEmpty(t, newTokenPair.RefreshToken)

		// Verify the tokens can be validated
		accessClaims, err := service.ValidateToken(newTokenPair.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, "access", accessClaims.Type)

		refreshClaims, err := service.ValidateToken(newTokenPair.RefreshToken)
		require.NoError(t, err)
		assert.Equal(t, "refresh", refreshClaims.Type)
	})

	t.Run("Extract User ID", func(t *testing.T) {
		tokenPair, err := service.GenerateTokenPair(user)
		require.NoError(t, err)

		userID, err := service.ExtractUserID(tokenPair.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, user.ID, userID)
	})

	t.Run("Token Type Checks", func(t *testing.T) {
		tokenPair, err := service.GenerateTokenPair(user)
		require.NoError(t, err)

		assert.True(t, service.IsAccessToken(tokenPair.AccessToken))
		assert.False(t, service.IsRefreshToken(tokenPair.AccessToken))

		assert.True(t, service.IsRefreshToken(tokenPair.RefreshToken))
		assert.False(t, service.IsAccessToken(tokenPair.RefreshToken))
	})

	t.Run("Invalid Token", func(t *testing.T) {
		invalidToken := "invalid.token.here"

		_, err := service.ValidateToken(invalidToken)
		assert.Error(t, err)

		_, err = service.ExtractUserID(invalidToken)
		assert.Error(t, err)

		assert.False(t, service.IsAccessToken(invalidToken))
		assert.False(t, service.IsRefreshToken(invalidToken))
	})

	t.Run("Wrong Secret Key", func(t *testing.T) {
		// Generate token with original service
		tokenPair, err := service.GenerateTokenPair(user)
		require.NoError(t, err)

		// Create service with different secret
		wrongConfig := *jwtConfig
		wrongConfig.SecretKey = "wrong-secret-key"
		wrongService := NewJWTService(&wrongConfig)

		// Try to validate with wrong service
		_, err = wrongService.ValidateToken(tokenPair.AccessToken)
		assert.Error(t, err)
	})
}

func TestJWTService_ErrorScenarios(t *testing.T) {
	jwtConfig := &config.JWTConfig{
		SecretKey:            "test-secret-key-for-testing-only",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: 7 * 24 * time.Hour,
		Issuer:               "voidrunner-test",
		Audience:             "voidrunner-api-test",
	}

	service := NewJWTService(jwtConfig)

	user := &models.User{
		BaseModel: models.BaseModel{
			ID: uuid.New(),
		},
		Email: "test@example.com",
	}

	t.Run("GenerateTokenPair with nil user", func(t *testing.T) {
		tokenPair, err := service.GenerateTokenPair(nil)
		assert.Error(t, err)
		assert.Nil(t, tokenPair)
	})

	t.Run("GenerateTokenPair with empty user ID", func(t *testing.T) {
		emptyUser := &models.User{
			BaseModel: models.BaseModel{
				ID: uuid.UUID{}, // Zero UUID
			},
			Email: "test@example.com",
		}

		tokenPair, err := service.GenerateTokenPair(emptyUser)
		assert.NoError(t, err) // Should still work but generate token with zero UUID
		assert.NotNil(t, tokenPair)
	})

	t.Run("GenerateTokenPair with empty email", func(t *testing.T) {
		emptyEmailUser := &models.User{
			BaseModel: models.BaseModel{
				ID: uuid.New(),
			},
			Email: "",
		}

		tokenPair, err := service.GenerateTokenPair(emptyEmailUser)
		assert.NoError(t, err) // Should still work but generate token with empty email
		assert.NotNil(t, tokenPair)
	})

	t.Run("ValidateToken with empty string", func(t *testing.T) {
		claims, err := service.ValidateToken("")
		assert.Error(t, err)
		assert.Nil(t, claims)
	})

	t.Run("ValidateToken with malformed JWT", func(t *testing.T) {
		malformedTokens := []string{
			"not-a-jwt-token",
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",           // Missing payload and signature
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.malformed", // Invalid base64
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ", // Missing signature
			"header.payload.signature.extra", // Too many parts
		}

		for _, token := range malformedTokens {
			claims, err := service.ValidateToken(token)
			assert.Error(t, err, "Should fail for token: %s", token)
			assert.Nil(t, claims)
		}
	})

	t.Run("ValidateToken with expired token", func(t *testing.T) {
		// Create a service with very short expiration
		shortConfig := *jwtConfig
		shortConfig.AccessTokenDuration = time.Nanosecond
		shortService := NewJWTService(&shortConfig)

		tokenPair, err := shortService.GenerateTokenPair(user)
		require.NoError(t, err)

		// Wait for token to expire
		time.Sleep(time.Millisecond * 10)

		claims, err := shortService.ValidateToken(tokenPair.AccessToken)
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "token is expired")
	})

	t.Run("RefreshToken with invalid token", func(t *testing.T) {
		invalidTokens := []string{
			"",
			"invalid-token",
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.malformed.signature",
		}

		for _, token := range invalidTokens {
			newTokenPair, err := service.RefreshToken(token)
			assert.Error(t, err, "Should fail for token: %s", token)
			assert.Nil(t, newTokenPair)
		}
	})

	t.Run("RefreshToken with access token instead of refresh token", func(t *testing.T) {
		tokenPair, err := service.GenerateTokenPair(user)
		require.NoError(t, err)

		// Try to refresh using access token
		newTokenPair, err := service.RefreshToken(tokenPair.AccessToken)
		assert.Error(t, err)
		assert.Nil(t, newTokenPair)
		assert.Contains(t, err.Error(), "token is not a refresh token")
	})

	t.Run("RefreshToken with expired refresh token", func(t *testing.T) {
		// Create a service with very short refresh token expiration
		shortConfig := *jwtConfig
		shortConfig.RefreshTokenDuration = time.Nanosecond
		shortService := NewJWTService(&shortConfig)

		tokenPair, err := shortService.GenerateTokenPair(user)
		require.NoError(t, err)

		// Wait for refresh token to expire
		time.Sleep(time.Millisecond * 10)

		newTokenPair, err := shortService.RefreshToken(tokenPair.RefreshToken)
		assert.Error(t, err)
		assert.Nil(t, newTokenPair)
	})

	t.Run("ExtractUserID with invalid tokens", func(t *testing.T) {
		invalidTokens := []string{
			"",
			"invalid-token",
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.malformed",
		}

		for _, token := range invalidTokens {
			userID, err := service.ExtractUserID(token)
			assert.Error(t, err, "Should fail for token: %s", token)
			assert.Equal(t, uuid.UUID{}, userID)
		}
	})

	t.Run("Token type checks with invalid tokens", func(t *testing.T) {
		invalidTokens := []string{
			"",
			"invalid-token",
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.malformed",
		}

		for _, token := range invalidTokens {
			assert.False(t, service.IsAccessToken(token))
			assert.False(t, service.IsRefreshToken(token))
		}
	})
}

func TestJWTService_EdgeCases(t *testing.T) {
	jwtConfig := &config.JWTConfig{
		SecretKey:            "test-secret-key-for-testing-only",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: 7 * 24 * time.Hour,
		Issuer:               "voidrunner-test",
		Audience:             "voidrunner-api-test",
	}

	service := NewJWTService(jwtConfig)

	t.Run("Very long user email", func(t *testing.T) {
		longEmailUser := &models.User{
			BaseModel: models.BaseModel{
				ID: uuid.New(),
			},
			Email: string(make([]byte, 500)) + "@example.com",
		}

		tokenPair, err := service.GenerateTokenPair(longEmailUser)
		assert.NoError(t, err)
		assert.NotNil(t, tokenPair)

		// Verify token can be validated
		claims, err := service.ValidateToken(tokenPair.AccessToken)
		assert.NoError(t, err)
		assert.Equal(t, longEmailUser.Email, claims.Email)
	})

	t.Run("Special characters in email", func(t *testing.T) {
		specialEmailUser := &models.User{
			BaseModel: models.BaseModel{
				ID: uuid.New(),
			},
			Email: "test+special@sub-domain.example-site.com",
		}

		tokenPair, err := service.GenerateTokenPair(specialEmailUser)
		assert.NoError(t, err)
		assert.NotNil(t, tokenPair)

		claims, err := service.ValidateToken(tokenPair.AccessToken)
		assert.NoError(t, err)
		assert.Equal(t, specialEmailUser.Email, claims.Email)
	})

	t.Run("Unicode characters in email", func(t *testing.T) {
		unicodeEmailUser := &models.User{
			BaseModel: models.BaseModel{
				ID: uuid.New(),
			},
			Email: "test@例え.テスト",
		}

		tokenPair, err := service.GenerateTokenPair(unicodeEmailUser)
		assert.NoError(t, err)
		assert.NotNil(t, tokenPair)

		claims, err := service.ValidateToken(tokenPair.AccessToken)
		assert.NoError(t, err)
		assert.Equal(t, unicodeEmailUser.Email, claims.Email)
	})

	t.Run("Token with future NotBefore time", func(t *testing.T) {
		// This tests edge case where token is valid but not yet active
		user := &models.User{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Email:     "test@example.com",
		}

		tokenPair, err := service.GenerateTokenPair(user)
		require.NoError(t, err)

		// Token should be immediately valid since NotBefore is set to current time
		claims, err := service.ValidateToken(tokenPair.AccessToken)
		assert.NoError(t, err)
		assert.NotNil(t, claims)
	})

	t.Run("Concurrent token operations", func(t *testing.T) {
		user := &models.User{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Email:     "test@example.com",
		}

		// Test concurrent token generation and validation
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()

				tokenPair, err := service.GenerateTokenPair(user)
				assert.NoError(t, err)
				assert.NotNil(t, tokenPair)

				claims, err := service.ValidateToken(tokenPair.AccessToken)
				assert.NoError(t, err)
				assert.Equal(t, user.ID, claims.UserID)
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}
