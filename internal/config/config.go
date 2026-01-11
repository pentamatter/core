package config

import (
	"os"
	"strconv"

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
	OAuthRedirectURL   string

	FrontendURL  string
	SecureCookie bool
	CookieDomain string // Cookie 域名，留空则使用当前请求域名
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
		OAuthRedirectURL:   getEnv("OAUTH_REDIRECT_URL", "http://localhost:8080/api/v1/auth/callback"),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:3000"),
		SecureCookie:       getEnv("SECURE_COOKIE", "false") == "true",
		CookieDomain:       getEnv("COOKIE_DOMAIN", ""), // 例如 ".example.com" 用于跨子域共享
	}
	return AppConfig
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
