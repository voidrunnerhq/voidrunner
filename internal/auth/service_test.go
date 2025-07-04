package auth

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// MockUserRepository is a mock implementation of UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepository) List(ctx context.Context, limit, offset int) ([]*models.User, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.User), args.Error(1)
}

func (m *MockUserRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func setupTestService() (*Service, *MockUserRepository, *JWTService) {
	mockRepo := &MockUserRepository{}
	
	cfg := &config.Config{
		JWT: config.JWTConfig{
			SecretKey:            "test-secret-key-for-testing-only",
			AccessTokenDuration:  15 * time.Minute,
			RefreshTokenDuration: 7 * 24 * time.Hour,
			Issuer:               "voidrunner-test",
			Audience:             "voidrunner-api-test",
		},
	}
	
	jwtService := NewJWTService(&cfg.JWT)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	service := NewService(mockRepo, jwtService, logger, cfg)
	
	return service, mockRepo, jwtService
}

func TestAuthService_Register(t *testing.T) {
	service, mockRepo, _ := setupTestService()
	ctx := context.Background()

	t.Run("Successful Registration", func(t *testing.T) {
		req := models.RegisterRequest{
			Email:    "test@example.com",
			Password: "TestPassword123!",
		}

		// Mock repository calls
		mockRepo.On("GetByEmail", ctx, req.Email).Return(nil, database.ErrUserNotFound).Once()
		mockRepo.On("Create", ctx, mock.AnythingOfType("*models.User")).Return(nil).Once()

		response, err := service.Register(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, response)

		assert.NotEmpty(t, response.AccessToken)
		assert.NotEmpty(t, response.RefreshToken)
		assert.Equal(t, "Bearer", response.TokenType)
		assert.Equal(t, req.Email, response.User.Email)

		mockRepo.AssertExpectations(t)
	})

	t.Run("User Already Exists", func(t *testing.T) {
		req := models.RegisterRequest{
			Email:    "existing@example.com",
			Password: "TestPassword123!",
		}

		existingUser := &models.User{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Email:     req.Email,
		}

		mockRepo.On("GetByEmail", ctx, req.Email).Return(existingUser, nil).Once()

		response, err := service.Register(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "already exists")

		mockRepo.AssertExpectations(t)
	})

	t.Run("Invalid Email", func(t *testing.T) {
		req := models.RegisterRequest{
			Email:    "invalid-email",
			Password: "TestPassword123!",
		}

		response, err := service.Register(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "invalid email")
	})

	t.Run("Invalid Password", func(t *testing.T) {
		req := models.RegisterRequest{
			Email:    "test@example.com",
			Password: "weak",
		}

		response, err := service.Register(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "invalid password")
	})
}

func TestAuthService_Login(t *testing.T) {
	service, mockRepo, _ := setupTestService()
	ctx := context.Background()

	t.Run("Successful Login", func(t *testing.T) {
		req := models.LoginRequest{
			Email:    "test@example.com",
			Password: "TestPassword123!",
		}

		// Hash the password for the mock user
		hashedPassword, err := service.hashPassword(req.Password)
		require.NoError(t, err)

		user := &models.User{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Email:     req.Email,
			PasswordHash: hashedPassword,
		}

		mockRepo.On("GetByEmail", ctx, req.Email).Return(user, nil).Once()

		response, err := service.Login(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, response)

		assert.NotEmpty(t, response.AccessToken)
		assert.NotEmpty(t, response.RefreshToken)
		assert.Equal(t, "Bearer", response.TokenType)
		assert.Equal(t, req.Email, response.User.Email)

		mockRepo.AssertExpectations(t)
	})

	t.Run("User Not Found", func(t *testing.T) {
		req := models.LoginRequest{
			Email:    "notfound@example.com",
			Password: "TestPassword123!",
		}

		mockRepo.On("GetByEmail", ctx, req.Email).Return(nil, database.ErrUserNotFound).Once()

		response, err := service.Login(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "invalid email or password")

		mockRepo.AssertExpectations(t)
	})

	t.Run("Wrong Password", func(t *testing.T) {
		req := models.LoginRequest{
			Email:    "test@example.com",
			Password: "WrongPassword123!",
		}

		// Hash a different password for the mock user
		hashedPassword, err := service.hashPassword("CorrectPassword123!")
		require.NoError(t, err)

		user := &models.User{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Email:     req.Email,
			PasswordHash: hashedPassword,
		}

		mockRepo.On("GetByEmail", ctx, req.Email).Return(user, nil).Once()

		response, err := service.Login(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "invalid email or password")

		mockRepo.AssertExpectations(t)
	})
}

func TestPasswordValidation(t *testing.T) {
	tests := []struct {
		name     string
		password string
		valid    bool
	}{
		{"Valid password", "TestPassword123!", true},
		{"Too short", "Test1!", false},
		{"No uppercase", "testpassword123!", false},
		{"No lowercase", "TESTPASSWORD123!", false},
		{"No digit", "TestPassword!", false},
		{"No special char", "TestPassword123", false},
		{"Too long", string(make([]byte, 130)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := models.ValidatePassword(tt.password)
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}