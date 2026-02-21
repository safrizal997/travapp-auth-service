package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/safrizal997/travapp-auth-service/internal/delivery/http/dto"
	"github.com/safrizal997/travapp-auth-service/internal/domain/repository"
	jwtpkg "github.com/safrizal997/travapp-auth-service/internal/pkg/jwt"
)

func AuthMiddleware(jwtManager *jwtpkg.Manager, tokenRepo repository.TokenRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error: "Missing authorization header",
				Code:  http.StatusUnauthorized,
			})
			return
		}

		var tokenString string
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			tokenString = authHeader
		}

		claims, err := jwtManager.ValidateAccessToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error: "Invalid or expired token",
				Code:  http.StatusUnauthorized,
			})
			return
		}

		blacklisted, err := tokenRepo.IsAccessTokenBlacklisted(c.Request.Context(), claims.JTI)
		if err != nil || blacklisted {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error: "Token has been revoked",
				Code:  http.StatusUnauthorized,
			})
			return
		}

		c.Set("credential_id", claims.Sub)
		c.Set("tenant_id", claims.TenantID)
		c.Set("role", claims.Role)
		c.Set("permissions", claims.Permissions)
		c.Set("provider", claims.Provider)
		c.Set("jti", claims.JTI)
		c.Set("token_exp", claims.ExpiresAt.Time)

		c.Next()
	}
}

func GetCredentialID(c *gin.Context) string {
	return c.GetString("credential_id")
}

func GetTenantID(c *gin.Context) string {
	return c.GetString("tenant_id")
}

func GetJTI(c *gin.Context) string {
	return c.GetString("jti")
}

func GetTokenExpiry(c *gin.Context) time.Time {
	exp, _ := c.Get("token_exp")
	if t, ok := exp.(time.Time); ok {
		return t
	}
	return time.Now()
}
