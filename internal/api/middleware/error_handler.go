package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()

			switch err.Type {
			case gin.ErrorTypeBind:
				c.JSON(http.StatusBadRequest, ErrorResponse{
					Error:   "Bad Request",
					Message: err.Error(),
					Code:    http.StatusBadRequest,
				})
			case gin.ErrorTypePublic:
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "Internal Server Error",
					Message: err.Error(),
					Code:    http.StatusInternalServerError,
				})
			default:
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "Internal Server Error",
					Message: "An unexpected error occurred",
					Code:    http.StatusInternalServerError,
				})
			}
		}
	}
}
