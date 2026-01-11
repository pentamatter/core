package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	MongoURI        string
	MongoDB         string
	MeilisearchHost string
	MeilisearchKey  string
	AdminEmail      string

	// Redis configuration
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	GitHubClientID     string
	GitHubClientSecret string
	GoogleClientID     string
	GoogleClientSecret string

	FrontendURL string // 前端域名，如 https://blog.example.com
	BackendURL  string // 后端域名，如 https://api.example.com
}

var AppConfig *Config

func Load() *Config {
	_ = godotenv.Load()

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))

	AppConfig = &Config{
		Port:               getEnv("PORT", "8080"),
		MongoURI:           getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:            getEnv("MONGO_DB", "matter_core"),
		MeilisearchHost:    getEnv("MEILISEARCH_HOST", "http://localhost:7700"),
		MeilisearchKey:     getEnv("MEILISEARCH_KEY", ""),
		AdminEmail:         getEnv("ADMIN_EMAIL", ""),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RedisDB:            redisDB,
		GitHubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:3000"),
		BackendURL:         getEnv("BACKEND_URL", "http://localhost:8080"),
	}
	return AppConfig
}

// GetAllowedOrigins 返回 CORS 允许的源
func (c *Config) GetAllowedOrigins() []string {
	return []string{c.FrontendURL}
}

// GetOAuthRedirectURL 返回 OAuth 回调地址
func (c *Config) GetOAuthRedirectURL() string {
	return c.BackendURL + "/api/v1/auth/callback"
}

// IsSecureCookie 根据 BackendURL 判断是否使用 Secure Cookie
func (c *Config) IsSecureCookie() bool {
	return strings.HasPrefix(c.BackendURL, "https://")
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
