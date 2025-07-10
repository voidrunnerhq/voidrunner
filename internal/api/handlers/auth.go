package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/voidrunnerhq/voidrunner/internal/auth"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	authService auth.AuthService
	logger      *slog.Logger
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(authService auth.AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		logger:      logger,
	}
}

// Register handles user registration
//
//	@Summary		Register a new user
//	@Description	Creates a new user account with email and password
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			request	body		models.RegisterRequest	true	"User registration details"
//	@Success		201		{object}	models.AuthResponse		"User registered successfully"
//	@Failure		400		{object}	models.ErrorResponse	"Invalid request format or validation error"
//	@Failure		409		{object}	models.ErrorResponse	"User already exists"
//	@Failure		429		{object}	models.ErrorResponse	"Rate limit exceeded"
//	@Router			/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid registration request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	authResponse, err := h.authService.Register(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("registration failed", "error", err)

		// Check if it's a duplicate user error
		if errors.Is(err, auth.ErrUserAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{
				"error": "User with this email already exists",
			})
			return
		}

		// Check if it's a validation error
		if errors.Is(err, auth.ErrInvalidEmail) ||
			errors.Is(err, auth.ErrInvalidPassword) ||
			errors.Is(err, auth.ErrValidationFailed) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Registration failed",
		})
		return
	}

	h.logger.Info("user registered successfully", "user_id", authResponse.User.ID)
	c.JSON(http.StatusCreated, authResponse)
}

// Login handles user login
//
//	@Summary		Authenticate user
//	@Description	Authenticates a user with email and password, returns access and refresh tokens
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			request	body		models.LoginRequest	true	"User login credentials"
//	@Success		200		{object}	models.AuthResponse		"Login successful"
//	@Failure		400		{object}	models.ErrorResponse	"Invalid request format or validation error"
//	@Failure		401		{object}	models.ErrorResponse	"Invalid credentials"
//	@Failure		429		{object}	models.ErrorResponse	"Rate limit exceeded"
//	@Router			/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid login request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	authResponse, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("login failed", "error", err)

		// Check if it's invalid credentials
		if errors.Is(err, auth.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid email or password",
			})
			return
		}

		// Check if it's a validation error
		if errors.Is(err, auth.ErrInvalidEmail) ||
			errors.Is(err, auth.ErrPasswordRequired) ||
			errors.Is(err, auth.ErrValidationFailed) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Login failed",
		})
		return
	}

	h.logger.Info("user logged in successfully", "user_id", authResponse.User.ID)
	c.JSON(http.StatusOK, authResponse)
}

// RefreshToken handles token refresh
//
//	@Summary		Refresh access token
//	@Description	Generates a new access token using a valid refresh token
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			request	body		models.RefreshTokenRequest	true	"Refresh token request"
//	@Success		200		{object}	models.AuthResponse			"Token refreshed successfully"
//	@Failure		400		{object}	models.ErrorResponse		"Invalid request format"
//	@Failure		401		{object}	models.ErrorResponse		"Invalid refresh token"
//	@Failure		429		{object}	models.ErrorResponse		"Rate limit exceeded"
//	@Router			/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req models.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid refresh token request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	authResponse, err := h.authService.RefreshToken(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("token refresh failed", "error", err)

		// Check if it's invalid token or user not found
		if errors.Is(err, auth.ErrInvalidRefreshToken) || errors.Is(err, auth.ErrUserNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid refresh token",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Token refresh failed",
		})
		return
	}

	h.logger.Info("token refreshed successfully", "user_id", authResponse.User.ID)
	c.JSON(http.StatusOK, authResponse)
}

// Me returns the current user's information
//
//	@Summary		Get current user
//	@Description	Returns information about the currently authenticated user
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	map[string]models.UserResponse	"User information retrieved successfully"
//	@Failure		401	{object}	models.ErrorResponse			"Unauthorized"
//	@Router			/auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	// Get user from context (set by auth middleware)
	user, exists := c.Get("user")
	if !exists {
		h.logger.Error("user not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	userModel, ok := user.(*models.User)
	if !ok {
		h.logger.Error("invalid user type in context")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": userModel.ToResponse(),
	})
}

// Logout handles user logout (client-side token removal)
//
//	@Summary		Logout user
//	@Description	Logs out the current user (client-side token removal)
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]string	"Logout successful"
//	@Router			/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// In a JWT system, logout is typically handled client-side
	// by removing the tokens from storage
	// Here we just return success
	c.JSON(http.StatusOK, gin.H{
		"message": "logged out successfully",
	})
}
