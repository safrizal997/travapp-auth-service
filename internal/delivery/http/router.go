package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/safrizal997/travapp-auth-service/internal/delivery/http/dto"
	"github.com/safrizal997/travapp-auth-service/internal/delivery/http/handler"
	"github.com/safrizal997/travapp-auth-service/internal/delivery/http/middleware"
	"github.com/safrizal997/travapp-auth-service/internal/domain/repository"
	jwtpkg "github.com/safrizal997/travapp-auth-service/internal/pkg/jwt"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type RouterDeps struct {
	AuthHandler     *handler.AuthHandler
	OAuthHandler    *handler.OAuthHandler
	TokenHandler    *handler.TokenHandler
	LogoutHandler   *handler.LogoutHandler
	PasswordHandler *handler.PasswordHandler
	JWTManager      *jwtpkg.Manager
	TokenRepo       repository.TokenRepository
	DBPool          *pgxpool.Pool
	RedisClient     *redis.Client
}

func NewRouter(deps *RouterDeps) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.CORS([]string{"*"}))
	router.Use(middleware.RateLimiter())

	// Swagger UI
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.GET("/health", deps.healthCheck)

	auth := router.Group("/api/v1/auth")
	{
		auth.POST("/login", deps.AuthHandler.Login)
		auth.POST("/register", deps.AuthHandler.Register)

		auth.GET("/oauth/google", deps.OAuthHandler.InitiateGoogle)
		auth.GET("/oauth/google/callback", deps.OAuthHandler.GoogleCallback)

		auth.POST("/token/refresh", deps.TokenHandler.RefreshToken)

		auth.GET("/verify-email", deps.PasswordHandler.VerifyEmail)

		auth.POST("/password/forgot", deps.PasswordHandler.ForgotPassword)
		auth.POST("/password/reset", deps.PasswordHandler.ResetPassword)

		auth.GET("/.well-known/jwks.json", deps.TokenHandler.JWKS)

		protected := auth.Group("")
		protected.Use(middleware.AuthMiddleware(deps.JWTManager, deps.TokenRepo))
		{
			protected.POST("/logout", deps.LogoutHandler.Logout)
			protected.POST("/logout/all", deps.LogoutHandler.LogoutAll)
			protected.POST("/password/change", deps.PasswordHandler.ChangePassword)
		}
	}

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error: "Route not found",
			Code:  http.StatusNotFound,
		})
	})

	return router
}

// healthCheck checks the health status of the service.
// @Summary Health check
// @Description Check the health status of the service, database, and Redis
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]interface{} "Service is healthy"
// @Failure 503 {object} map[string]interface{} "Service is unhealthy"
// @Router /health [get]
func (deps *RouterDeps) healthCheck(c *gin.Context) {
	dbErr := deps.DBPool.Ping(c.Request.Context())
	redisErr := deps.RedisClient.Ping(c.Request.Context()).Err()

	if dbErr != nil || redisErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":   "unhealthy",
			"database": dbErr == nil,
			"redis":    redisErr == nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "healthy",
		"database": true,
		"redis":    true,
	})
}
