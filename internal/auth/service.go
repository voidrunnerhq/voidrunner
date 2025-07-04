package auth

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/crypto/bcrypt"

	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// Service handles authentication operations
type Service struct {
	userRepo database.UserRepository
	jwtSvc   *JWTService
	logger   *slog.Logger
	config   *config.Config
}

// NewService creates a new authentication service
func NewService(
	userRepo database.UserRepository,
	jwtSvc *JWTService,
	logger *slog.Logger,
	config *config.Config,
) *Service {
	return &Service{
		userRepo: userRepo,
		jwtSvc:   jwtSvc,
		logger:   logger,
		config:   config,
	}
}

// Register registers a new user
func (s *Service) Register(ctx context.Context, req models.RegisterRequest) (*models.AuthResponse, error) {
	logger := s.logger.With(
		"operation", "register",
		"email", req.Email,
	)

	logger.Info("attempting user registration")

	// Validate input
	if err := models.ValidateEmail(req.Email); err != nil {
		logger.Warn("invalid email format", "error", err)
		return nil, fmt.Errorf("invalid email: %w", err)
	}

	if err := models.ValidatePassword(req.Password); err != nil {
		logger.Warn("invalid password format", "error", err)
		return nil, fmt.Errorf("invalid password: %w", err)
	}

	// Check if user already exists
	existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil && err != database.ErrUserNotFound {
		logger.Error("failed to check existing user", "error", err)
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	if existingUser != nil {
		logger.Warn("user already exists")
		return nil, fmt.Errorf("user with email %s already exists", req.Email)
	}

	// Hash password
	passwordHash, err := s.hashPassword(req.Password)
	if err != nil {
		logger.Error("failed to hash password", "error", err)
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &models.User{
		Email:        req.Email,
		PasswordHash: passwordHash,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		logger.Error("failed to create user", "error", err)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	logger.Info("user registered successfully", "user_id", user.ID)

	// Generate tokens
	tokenPair, err := s.jwtSvc.GenerateTokenPair(user)
	if err != nil {
		logger.Error("failed to generate tokens", "error", err)
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	return &models.AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    tokenPair.ExpiresIn,
		User:         user.ToResponse(),
	}, nil
}

// Login authenticates a user and returns tokens
func (s *Service) Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
	logger := s.logger.With(
		"operation", "login",
		"email", req.Email,
	)

	logger.Info("attempting user login")

	// Validate input
	if err := models.ValidateEmail(req.Email); err != nil {
		logger.Warn("invalid email format", "error", err)
		return nil, fmt.Errorf("invalid email: %w", err)
	}

	if req.Password == "" {
		logger.Warn("empty password")
		return nil, fmt.Errorf("password is required")
	}

	// Get user by email
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if err == database.ErrUserNotFound {
			logger.Warn("user not found")
			return nil, fmt.Errorf("invalid email or password")
		}
		logger.Error("failed to get user", "error", err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Verify password
	if err := s.verifyPassword(req.Password, user.PasswordHash); err != nil {
		logger.Warn("invalid password")
		return nil, fmt.Errorf("invalid email or password")
	}

	logger.Info("user logged in successfully", "user_id", user.ID)

	// Generate tokens
	tokenPair, err := s.jwtSvc.GenerateTokenPair(user)
	if err != nil {
		logger.Error("failed to generate tokens", "error", err)
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	return &models.AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    tokenPair.ExpiresIn,
		User:         user.ToResponse(),
	}, nil
}

// RefreshToken generates new tokens from a refresh token
func (s *Service) RefreshToken(ctx context.Context, req models.RefreshTokenRequest) (*models.AuthResponse, error) {
	logger := s.logger.With(
		"operation", "refresh_token",
	)

	logger.Info("attempting token refresh")

	// Validate and generate new tokens
	tokenPair, err := s.jwtSvc.RefreshToken(req.RefreshToken)
	if err != nil {
		logger.Warn("invalid refresh token", "error", err)
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Extract user ID from the new access token to get updated user info
	userID, err := s.jwtSvc.ExtractUserID(tokenPair.AccessToken)
	if err != nil {
		logger.Error("failed to extract user ID from token", "error", err)
		return nil, fmt.Errorf("failed to extract user ID: %w", err)
	}

	// Get user data
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if err == database.ErrUserNotFound {
			logger.Warn("user not found for refresh token", "user_id", userID)
			return nil, fmt.Errorf("user not found")
		}
		logger.Error("failed to get user for refresh", "error", err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	logger.Info("token refreshed successfully", "user_id", user.ID)

	return &models.AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    tokenPair.ExpiresIn,
		User:         user.ToResponse(),
	}, nil
}

// ValidateAccessToken validates an access token and returns the user
func (s *Service) ValidateAccessToken(ctx context.Context, tokenString string) (*models.User, error) {
	claims, err := s.jwtSvc.ValidateToken(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Verify it's an access token
	if claims.Type != "access" {
		return nil, fmt.Errorf("token is not an access token")
	}

	// Get user data
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		if err == database.ErrUserNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// hashPassword hashes a password using bcrypt
func (s *Service) hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// verifyPassword verifies a password against a hash
func (s *Service) verifyPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}