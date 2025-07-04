package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/voidrunnerhq/voidrunner/internal/auth"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	authService *auth.Service
	logger      *slog.Logger
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(authService *auth.Service, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		logger:      logger,
	}
}

// Register handles user registration
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
		h.logger.Error("registration failed", "error", err, "email", req.Email)
		
		// Check if it's a validation error or duplicate user
		if err.Error() == "user with email "+req.Email+" already exists" {
			c.JSON(http.StatusConflict, gin.H{
				"error": "User with this email already exists",
			})
			return
		}
		
		// Check if it's a validation error
		if err.Error() == "invalid email" || err.Error() == "invalid password" {
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
		h.logger.Error("login failed", "error", err, "email", req.Email)
		
		// Check if it's invalid credentials
		if err.Error() == "invalid email or password" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid email or password",
			})
			return
		}
		
		// Check if it's a validation error
		if err.Error() == "invalid email" || err.Error() == "password is required" {
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
		
		// Check if it's invalid token
		if err.Error() == "invalid refresh token" || err.Error() == "user not found" {
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
func (h *AuthHandler) Logout(c *gin.Context) {
	// In a JWT system, logout is typically handled client-side
	// by removing the tokens from storage
	// Here we just return success
	c.JSON(http.StatusOK, gin.H{
		"message": "Successfully logged out",
	})
}