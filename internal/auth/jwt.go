package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// JWTService handles JWT token operations
type JWTService struct {
	config *config.JWTConfig
}

// NewJWTService creates a new JWT service
func NewJWTService(config *config.JWTConfig) *JWTService {
	return &JWTService{
		config: config,
	}
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

// GenerateTokenPair generates both access and refresh tokens for a user
func (s *JWTService) GenerateTokenPair(user *models.User) (*TokenPair, error) {
	if user == nil {
		return nil, fmt.Errorf("user cannot be nil")
	}

	// Generate access token
	accessToken, err := s.generateToken(user, "access", s.config.AccessTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := s.generateToken(user, "refresh", s.config.RefreshTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.config.AccessTokenDuration.Seconds()),
	}, nil
}

// generateToken generates a JWT token for a user
func (s *JWTService) generateToken(user *models.User, tokenType string, duration time.Duration) (string, error) {
	expiresAt := time.Now().Add(duration)

	claims := user.ToJWTClaims(tokenType, s.config.Issuer, s.config.Audience, expiresAt)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(s.config.SecretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func (s *JWTService) ValidateToken(tokenString string) (*models.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &models.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.SecretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*models.JWTClaims); ok && token.Valid {
		// Verify issuer and audience
		if claims.Issuer != s.config.Issuer {
			return nil, fmt.Errorf("invalid issuer")
		}

		if len(claims.Audience) == 0 || claims.Audience[0] != s.config.Audience {
			return nil, fmt.Errorf("invalid audience")
		}

		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// RefreshToken generates a new access token from a valid refresh token
func (s *JWTService) RefreshToken(refreshTokenString string) (*TokenPair, error) {
	claims, err := s.ValidateToken(refreshTokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Verify it's a refresh token
	if claims.Type != "refresh" {
		return nil, fmt.Errorf("token is not a refresh token")
	}

	// Create a user object from claims for token generation
	user := &models.User{
		BaseModel: models.BaseModel{
			ID: claims.UserID,
		},
		Email: claims.Email,
	}

	// Generate new token pair
	tokenPair, err := s.GenerateTokenPair(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new token pair: %w", err)
	}

	return tokenPair, nil
}

// ExtractUserID extracts the user ID from a token string
func (s *JWTService) ExtractUserID(tokenString string) (uuid.UUID, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return uuid.Nil, err
	}

	return claims.UserID, nil
}

// IsAccessToken checks if the token is an access token
func (s *JWTService) IsAccessToken(tokenString string) bool {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return false
	}

	return claims.Type == "access"
}

// IsRefreshToken checks if the token is a refresh token
func (s *JWTService) IsRefreshToken(tokenString string) bool {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return false
	}

	return claims.Type == "refresh"
}
