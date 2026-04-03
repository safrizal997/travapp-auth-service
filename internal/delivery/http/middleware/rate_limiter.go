package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/safrizal997/travapp-auth-service/internal/delivery/http/dto"
)

func RateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func AbortWithTooManyRequests(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusTooManyRequests, dto.ErrorResponse{
		Error: "Too many requests. Please try again later.",
		Code:  http.StatusTooManyRequests,
	})
}
