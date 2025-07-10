package auth

import (
	"context"

	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// AuthService interface for authentication operations
type AuthService interface {
	Register(ctx context.Context, req models.RegisterRequest) (*models.AuthResponse, error)
	Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error)
	RefreshToken(ctx context.Context, req models.RefreshTokenRequest) (*models.AuthResponse, error)
	ValidateAccessToken(ctx context.Context, token string) (*models.User, error)
}

// Ensure Service implements AuthService
var _ AuthService = (*Service)(nil)
