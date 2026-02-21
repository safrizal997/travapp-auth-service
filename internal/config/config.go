package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App       AppConfig
	DB        DBConfig
	Redis     RedisConfig
	JWT       JWTConfig
	Google    GoogleConfig
	BCrypt    BCryptConfig
	RateLimit RateLimitConfig
}

type AppConfig struct {
	Port    string
	Env     string
	BaseURL string
}

type DBConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func (c *DBConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode)
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

func (c *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

type JWTConfig struct {
	PrivateKeyPath  string
	PublicKeyPath   string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

type BCryptConfig struct {
	Cost int
}

type RateLimitConfig struct {
	LoginMax    int
	LoginWindow time.Duration
	ResetMax    int
	ResetWindow time.Duration
}

func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	// Ignore error if .env file doesn't exist (env vars may be set directly)
	_ = viper.ReadInConfig()

	viper.SetDefault("APP_PORT", "8080")
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("APP_BASE_URL", "http://localhost:8080")
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", "5432")
	viper.SetDefault("DB_SSL_MODE", "disable")
	viper.SetDefault("DB_MAX_OPEN_CONNS", 25)
	viper.SetDefault("DB_MAX_IDLE_CONNS", 5)
	viper.SetDefault("DB_CONN_MAX_LIFETIME", 300)
	viper.SetDefault("REDIS_HOST", "localhost")
	viper.SetDefault("REDIS_PORT", "6379")
	viper.SetDefault("REDIS_DB", 1)
	viper.SetDefault("JWT_ACCESS_TOKEN_TTL", 900)
	viper.SetDefault("JWT_REFRESH_TOKEN_TTL", 2592000)
	viper.SetDefault("BCRYPT_COST", 12)
	viper.SetDefault("RATE_LIMIT_LOGIN_MAX", 5)
	viper.SetDefault("RATE_LIMIT_LOGIN_WINDOW", 900)
	viper.SetDefault("RATE_LIMIT_RESET_MAX", 3)
	viper.SetDefault("RATE_LIMIT_RESET_WINDOW", 3600)

	cfg := &Config{
		App: AppConfig{
			Port:    viper.GetString("APP_PORT"),
			Env:     viper.GetString("APP_ENV"),
			BaseURL: viper.GetString("APP_BASE_URL"),
		},
		DB: DBConfig{
			Host:            viper.GetString("DB_HOST"),
			Port:            viper.GetString("DB_PORT"),
			User:            viper.GetString("DB_USER"),
			Password:        viper.GetString("DB_PASSWORD"),
			Name:            viper.GetString("DB_NAME"),
			SSLMode:         viper.GetString("DB_SSL_MODE"),
			MaxOpenConns:    viper.GetInt("DB_MAX_OPEN_CONNS"),
			MaxIdleConns:    viper.GetInt("DB_MAX_IDLE_CONNS"),
			ConnMaxLifetime: time.Duration(viper.GetInt("DB_CONN_MAX_LIFETIME")) * time.Second,
		},
		Redis: RedisConfig{
			Host:     viper.GetString("REDIS_HOST"),
			Port:     viper.GetString("REDIS_PORT"),
			Password: viper.GetString("REDIS_PASSWORD"),
			DB:       viper.GetInt("REDIS_DB"),
		},
		JWT: JWTConfig{
			PrivateKeyPath:  viper.GetString("JWT_PRIVATE_KEY_PATH"),
			PublicKeyPath:   viper.GetString("JWT_PUBLIC_KEY_PATH"),
			AccessTokenTTL:  time.Duration(viper.GetInt("JWT_ACCESS_TOKEN_TTL")) * time.Second,
			RefreshTokenTTL: time.Duration(viper.GetInt("JWT_REFRESH_TOKEN_TTL")) * time.Second,
		},
		Google: GoogleConfig{
			ClientID:     viper.GetString("GOOGLE_CLIENT_ID"),
			ClientSecret: viper.GetString("GOOGLE_CLIENT_SECRET"),
			RedirectURI:  viper.GetString("GOOGLE_REDIRECT_URI"),
		},
		BCrypt: BCryptConfig{
			Cost: viper.GetInt("BCRYPT_COST"),
		},
		RateLimit: RateLimitConfig{
			LoginMax:    viper.GetInt("RATE_LIMIT_LOGIN_MAX"),
			LoginWindow: time.Duration(viper.GetInt("RATE_LIMIT_LOGIN_WINDOW")) * time.Second,
			ResetMax:    viper.GetInt("RATE_LIMIT_RESET_MAX"),
			ResetWindow: time.Duration(viper.GetInt("RATE_LIMIT_RESET_WINDOW")) * time.Second,
		},
	}

	return cfg, nil
}
