package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/safrizal997/travapp-auth-service/internal/config"
	delivery "github.com/safrizal997/travapp-auth-service/internal/delivery/http"
	"github.com/safrizal997/travapp-auth-service/internal/delivery/http/handler"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/hash"
	jwtpkg "github.com/safrizal997/travapp-auth-service/internal/pkg/jwt"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/oauth"
	pgRepo "github.com/safrizal997/travapp-auth-service/internal/repository/postgres"
	redisRepo "github.com/safrizal997/travapp-auth-service/internal/repository/redis"
	"github.com/safrizal997/travapp-auth-service/internal/usecase"

	_ "github.com/safrizal997/travapp-auth-service/docs"
)

// @title TravApp Auth Service API
// @version 1.0
// @description Authentication and authorization microservice for TravApp platform. Provides user registration, login, OAuth2 (Google), JWT token management, password management, and email verification.

// @host localhost:8080
// @BasePath /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Paste access_token saja (tanpa prefix Bearer), otomatis ditambahkan.

func main() {
	// Logger
	var logger *zap.Logger
	var err error

	cfg, err := config.Load()
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}

	if cfg.App.Env == "production" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	// Database
	ctx := context.Background()
	poolConfig, err := pgxpool.ParseConfig(cfg.DB.DSN())
	if err != nil {
		logger.Fatal("Failed to parse database config", zap.Error(err))
	}
	poolConfig.MaxConns = int32(cfg.DB.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.DB.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.DB.ConnMaxLifetime

	dbPool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		logger.Fatal("Failed to ping database", zap.Error(err))
	}
	logger.Info("Connected to PostgreSQL")

	// Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	logger.Info("Connected to Redis")

	// JWT Manager
	jwtManager, err := jwtpkg.NewManager(cfg.JWT.PrivateKeyPath, cfg.JWT.PublicKeyPath)
	if err != nil {
		logger.Fatal("Failed to initialize JWT manager", zap.Error(err))
	}

	// Hasher
	hasher := hash.NewBCryptHasher(cfg.BCrypt.Cost)

	// Google OAuth Config
	googleConfig := &oauth.GoogleConfig{
		ClientID:     cfg.Google.ClientID,
		ClientSecret: cfg.Google.ClientSecret,
		RedirectURI:  cfg.Google.RedirectURI,
	}

	// Repositories
	credentialRepo := pgRepo.NewCredentialRepo(dbPool)
	tenantRepo := pgRepo.NewTenantRepo(dbPool)
	roleRepo := pgRepo.NewRoleRepo(dbPool)
	loginHistRepo := pgRepo.NewLoginHistoryRepo(dbPool)
	passwordResetRepo := pgRepo.NewPasswordResetRepo(dbPool)
	tokenRepo := redisRepo.NewTokenRepo(redisClient)

	// Use Cases
	loginUC := usecase.NewLoginUseCase(
		credentialRepo, tenantRepo, roleRepo, tokenRepo, loginHistRepo,
		hasher, jwtManager, logger,
		cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL,
		cfg.RateLimit.LoginMax, cfg.RateLimit.LoginWindow,
	)

	registerUC := usecase.NewRegisterUseCase(
		credentialRepo, tenantRepo, roleRepo, hasher, logger,
		passwordResetRepo, cfg.App.BaseURL,
	)

	oauthUC := usecase.NewOAuthUseCase(
		credentialRepo, tenantRepo, roleRepo, tokenRepo, loginHistRepo,
		googleConfig, jwtManager, logger,
		cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL,
	)

	tokenUC := usecase.NewTokenUseCase(
		credentialRepo, roleRepo, tokenRepo, jwtManager, logger,
		cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL,
	)

	logoutUC := usecase.NewLogoutUseCase(tokenRepo, loginHistRepo, logger)

	passwordUC := usecase.NewPasswordUseCase(
		credentialRepo, tenantRepo, tokenRepo, hasher, logger, passwordResetRepo,
		cfg.App.BaseURL,
	)

	// Handlers
	authHandler := handler.NewAuthHandler(loginUC, registerUC, logger)
	oauthHandler := handler.NewOAuthHandler(oauthUC, logger)
	tokenHandler := handler.NewTokenHandler(tokenUC, jwtManager, logger)
	logoutHandler := handler.NewLogoutHandler(logoutUC, logger)
	passwordHandler := handler.NewPasswordHandler(passwordUC, logger)

	// Router
	router := delivery.NewRouter(&delivery.RouterDeps{
		AuthHandler:     authHandler,
		OAuthHandler:    oauthHandler,
		TokenHandler:    tokenHandler,
		LogoutHandler:   logoutHandler,
		PasswordHandler: passwordHandler,
		JWTManager:      jwtManager,
		TokenRepo:       tokenRepo,
		DBPool:          dbPool,
		RedisClient:     redisClient,
	})

	// Server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.App.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		logger.Info("Starting server", zap.String("port", cfg.App.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited gracefully")
}
