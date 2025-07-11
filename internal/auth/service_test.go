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
			Name:     "Test User",
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
		assert.Equal(t, req.Name, response.User.Name)

		mockRepo.AssertExpectations(t)
	})

	t.Run("User Already Exists", func(t *testing.T) {
		req := models.RegisterRequest{
			Email:    "existing@example.com",
			Password: "TestPassword123!",
			Name:     "Existing User",
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
			Name:     "Test User",
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
			Name:     "Test User",
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
			BaseModel:    models.BaseModel{ID: uuid.New()},
			Email:        req.Email,
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
			BaseModel:    models.BaseModel{ID: uuid.New()},
			Email:        req.Email,
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

func TestAuthService_RefreshToken(t *testing.T) {
	service, mockRepo, jwtService := setupTestService()
	ctx := context.Background()

	// Create a test user and generate tokens
	user := &models.User{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Email:     "test@example.com",
		Name:      "Test User",
	}

	tokenPair, err := jwtService.GenerateTokenPair(user)
	require.NoError(t, err)

	t.Run("Successful Token Refresh", func(t *testing.T) {
		req := models.RefreshTokenRequest{
			RefreshToken: tokenPair.RefreshToken,
		}

		mockRepo.On("GetByID", ctx, user.ID).Return(user, nil).Once()

		response, err := service.RefreshToken(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, response)

		assert.NotEmpty(t, response.AccessToken)
		assert.NotEmpty(t, response.RefreshToken)
		assert.Equal(t, "Bearer", response.TokenType)
		assert.Equal(t, user.Email, response.User.Email)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Invalid Refresh Token", func(t *testing.T) {
		req := models.RefreshTokenRequest{
			RefreshToken: "invalid-token",
		}

		response, err := service.RefreshToken(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "invalid refresh token")
	})

	t.Run("User Not Found After Token Refresh", func(t *testing.T) {
		req := models.RefreshTokenRequest{
			RefreshToken: tokenPair.RefreshToken,
		}

		mockRepo.On("GetByID", ctx, user.ID).Return(nil, database.ErrUserNotFound).Once()

		response, err := service.RefreshToken(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "user not found")

		mockRepo.AssertExpectations(t)
	})

	t.Run("Database Error During User Lookup", func(t *testing.T) {
		req := models.RefreshTokenRequest{
			RefreshToken: tokenPair.RefreshToken,
		}

		mockRepo.On("GetByID", ctx, user.ID).Return(nil, assert.AnError).Once()

		response, err := service.RefreshToken(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "failed to get user")

		mockRepo.AssertExpectations(t)
	})

	t.Run("Empty Refresh Token", func(t *testing.T) {
		req := models.RefreshTokenRequest{
			RefreshToken: "",
		}

		response, err := service.RefreshToken(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "invalid refresh token")
	})

	t.Run("Malformed Refresh Token", func(t *testing.T) {
		req := models.RefreshTokenRequest{
			RefreshToken: "malformed.token.here",
		}

		response, err := service.RefreshToken(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "invalid refresh token")
	})
}

func TestAuthService_ValidateAccessToken(t *testing.T) {
	service, mockRepo, jwtService := setupTestService()
	ctx := context.Background()

	// Create a test user and generate tokens
	user := &models.User{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Email:     "test@example.com",
		Name:      "Test User",
	}

	tokenPair, err := jwtService.GenerateTokenPair(user)
	require.NoError(t, err)

	t.Run("Valid Access Token", func(t *testing.T) {
		mockRepo.On("GetByID", ctx, user.ID).Return(user, nil).Once()

		validatedUser, err := service.ValidateAccessToken(ctx, tokenPair.AccessToken)
		require.NoError(t, err)
		require.NotNil(t, validatedUser)

		assert.Equal(t, user.ID, validatedUser.ID)
		assert.Equal(t, user.Email, validatedUser.Email)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Invalid Token Format", func(t *testing.T) {
		invalidToken := "invalid-token-format"

		validatedUser, err := service.ValidateAccessToken(ctx, invalidToken)
		assert.Error(t, err)
		assert.Nil(t, validatedUser)
		assert.Contains(t, err.Error(), "invalid token")
	})

	t.Run("Refresh Token Instead of Access Token", func(t *testing.T) {
		validatedUser, err := service.ValidateAccessToken(ctx, tokenPair.RefreshToken)
		assert.Error(t, err)
		assert.Nil(t, validatedUser)
		assert.Contains(t, err.Error(), "token is not an access token")
	})

	t.Run("User Not Found", func(t *testing.T) {
		mockRepo.On("GetByID", ctx, user.ID).Return(nil, database.ErrUserNotFound).Once()

		validatedUser, err := service.ValidateAccessToken(ctx, tokenPair.AccessToken)
		assert.Error(t, err)
		assert.Nil(t, validatedUser)
		assert.Contains(t, err.Error(), "user not found")

		mockRepo.AssertExpectations(t)
	})

	t.Run("Database Error During User Lookup", func(t *testing.T) {
		mockRepo.On("GetByID", ctx, user.ID).Return(nil, assert.AnError).Once()

		validatedUser, err := service.ValidateAccessToken(ctx, tokenPair.AccessToken)
		assert.Error(t, err)
		assert.Nil(t, validatedUser)
		assert.Contains(t, err.Error(), "failed to get user")

		mockRepo.AssertExpectations(t)
	})

	t.Run("Empty Token", func(t *testing.T) {
		validatedUser, err := service.ValidateAccessToken(ctx, "")
		assert.Error(t, err)
		assert.Nil(t, validatedUser)
		assert.Contains(t, err.Error(), "invalid token")
	})

	t.Run("Malformed JWT Token", func(t *testing.T) {
		// #nosec G101 - This is a test token, not actual credentials
		malformedToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.malformed"

		validatedUser, err := service.ValidateAccessToken(ctx, malformedToken)
		assert.Error(t, err)
		assert.Nil(t, validatedUser)
		assert.Contains(t, err.Error(), "invalid token")
	})
}

func TestAuthService_Register_ErrorScenarios(t *testing.T) {
	service, mockRepo, _ := setupTestService()
	ctx := context.Background()

	t.Run("Database Error Checking Existing User", func(t *testing.T) {
		req := models.RegisterRequest{
			Email:    "test@example.com",
			Password: "TestPassword123!",
			Name:     "Test User",
		}

		mockRepo.On("GetByEmail", ctx, req.Email).Return(nil, assert.AnError).Once()

		response, err := service.Register(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "failed to check existing user")

		mockRepo.AssertExpectations(t)
	})

	t.Run("Database Error Creating User", func(t *testing.T) {
		req := models.RegisterRequest{
			Email:    "test@example.com",
			Password: "TestPassword123!",
			Name:     "Test User",
		}

		mockRepo.On("GetByEmail", ctx, req.Email).Return(nil, database.ErrUserNotFound).Once()
		mockRepo.On("Create", ctx, mock.AnythingOfType("*models.User")).Return(assert.AnError).Once()

		response, err := service.Register(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "failed to create user")

		mockRepo.AssertExpectations(t)
	})

	t.Run("Invalid Name - Empty", func(t *testing.T) {
		req := models.RegisterRequest{
			Email:    "test@example.com",
			Password: "TestPassword123!",
			Name:     "",
		}

		response, err := service.Register(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "validation error")
	})

	t.Run("Invalid Name - Too Long", func(t *testing.T) {
		req := models.RegisterRequest{
			Email:    "test@example.com",
			Password: "TestPassword123!",
			Name:     string(make([]byte, 300)), // > 255 characters
		}

		response, err := service.Register(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "validation error")
	})

	t.Run("Invalid Name - Whitespace Only", func(t *testing.T) {
		req := models.RegisterRequest{
			Email:    "test@example.com",
			Password: "TestPassword123!",
			Name:     "   \t\n  ",
		}

		response, err := service.Register(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "validation error")
	})
}

func TestAuthService_Login_ErrorScenarios(t *testing.T) {
	service, mockRepo, _ := setupTestService()
	ctx := context.Background()

	t.Run("Database Error Getting User", func(t *testing.T) {
		req := models.LoginRequest{
			Email:    "test@example.com",
			Password: "TestPassword123!",
		}

		mockRepo.On("GetByEmail", ctx, req.Email).Return(nil, assert.AnError).Once()

		response, err := service.Login(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "failed to get user")

		mockRepo.AssertExpectations(t)
	})

	t.Run("Empty Password", func(t *testing.T) {
		req := models.LoginRequest{
			Email:    "test@example.com",
			Password: "",
		}

		response, err := service.Login(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "password is required")
	})

	t.Run("Invalid Email Format", func(t *testing.T) {
		req := models.LoginRequest{
			Email:    "not-an-email",
			Password: "TestPassword123!",
		}

		response, err := service.Login(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "invalid email")
	})
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid name",
			input:   "John Doe",
			wantErr: false,
		},
		{
			name:    "empty name",
			input:   "",
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name:    "whitespace only name",
			input:   "   \t\n  ",
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name:    "name too long",
			input:   string(make([]byte, 300)),
			wantErr: true,
			errMsg:  "name is too long",
		},
		{
			name:    "valid single character name",
			input:   "A",
			wantErr: false,
		},
		{
			name:    "valid name with numbers",
			input:   "John Doe 123",
			wantErr: false,
		},
		{
			name:    "valid name with special characters",
			input:   "John O'Connor-Smith",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateName(tt.input)
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

func TestHashPassword_ErrorScenarios(t *testing.T) {
	service, _, _ := setupTestService()

	t.Run("Hash Valid Password", func(t *testing.T) {
		password := "TestPassword123!"
		hash, err := service.hashPassword(password)
		assert.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.NotEqual(t, password, hash)
	})

	t.Run("Hash Empty Password", func(t *testing.T) {
		hash, err := service.hashPassword("")
		assert.NoError(t, err) // bcrypt can hash empty strings
		assert.NotEmpty(t, hash)
	})

	t.Run("Hash Very Long Password", func(t *testing.T) {
		// bcrypt has a limit of 72 bytes for passwords
		longPassword := string(make([]byte, 100))
		for i := range longPassword {
			longPassword = longPassword[:i] + "a" + longPassword[i+1:]
		}

		hash, err := service.hashPassword(longPassword)
		assert.Error(t, err) // bcrypt errors on passwords > 72 bytes
		assert.Empty(t, hash)
		assert.Contains(t, err.Error(), "password length exceeds 72 bytes")
	})
}
