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