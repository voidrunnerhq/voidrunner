package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/voidrunnerhq/voidrunner/internal/auth"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// AuthMiddleware handles JWT authentication
type AuthMiddleware struct {
	authService auth.AuthService
	logger      *slog.Logger
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(authService auth.AuthService, logger *slog.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
		logger:      logger,
	}
}

// RequireAuth middleware that requires authentication
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := m.extractToken(c)
		if token == "" {
			m.logger.Warn("missing or invalid authorization header")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "authorization header required",
			})
			c.Abort()
			return
		}

		user, err := m.authService.ValidateAccessToken(c.Request.Context(), token)
		if err != nil {
			m.logger.Warn("invalid access token", "error", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Set user in context for use in handlers
		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Set("user_email", user.Email)

		c.Next()
	}
}

// OptionalAuth middleware that adds user info if token is present and valid
func (m *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := m.extractToken(c)
		if token == "" {
			// No token provided, continue without auth
			c.Next()
			return
		}

		user, err := m.authService.ValidateAccessToken(c.Request.Context(), token)
		if err != nil {
			// Invalid token, continue without auth but log warning
			m.logger.Warn("invalid access token in optional auth", "error", err)
			c.Next()
			return
		}

		// Set user in context for use in handlers
		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Set("user_email", user.Email)

		c.Next()
	}
}

// extractToken extracts the JWT token from the Authorization header
func (m *AuthMiddleware) extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}

	// Check for Bearer token format
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return ""
	}

	if parts[0] != "Bearer" {
		return ""
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return ""
	}

	return token
}

// GetUserFromContext safely extracts user from gin context
func GetUserFromContext(c *gin.Context) *models.User {
	user, exists := c.Get("user")
	if !exists {
		return nil
	}

	userModel, ok := user.(*models.User)
	if !ok {
		return nil
	}

	return userModel
}

// RequireUserID middleware that ensures the user can only access their own resources
func (m *AuthMiddleware) RequireUserID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		user, exists := c.Get("user")
		if !exists {
			m.logger.Warn("user not found in context")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized",
			})
			c.Abort()
			return
		}

		// Get user ID from URL parameter
		userIDParam := c.Param("user_id")
		if userIDParam == "" {
			m.logger.Warn("user_id parameter not found")
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "User ID parameter required",
			})
			c.Abort()
			return
		}

		// Get authenticated user
		userModel, ok := user.(*models.User)
		if !ok {
			m.logger.Error("invalid user type in context")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Internal server error",
			})
			c.Abort()
			return
		}

		// Check if user is accessing their own resources
		if userModel.ID.String() != userIDParam {
			m.logger.Warn("user attempting to access another user's resources",
				"authenticated_user_id", userModel.ID,
				"requested_user_id", userIDParam,
			)
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
