package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	MongoURI        string
	MongoDB         string
	MeilisearchHost string
	MeilisearchKey  string
	JWTSecret       string
	AdminEmail      string

	GitHubClientID     string
	GitHubClientSecret string
	GoogleClientID     string
	GoogleClientSecret string
	OAuthRedirectURL   string
}

var AppConfig *Config

func Load() *Config {
	_ = godotenv.Load()

	AppConfig = &Config{
		Port:               getEnv("PORT", "8080"),
		MongoURI:           getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:            getEnv("MONGO_DB", "matter_core"),
		MeilisearchHost:    getEnv("MEILISEARCH_HOST", "http://localhost:7700"),
		MeilisearchKey:     getEnv("MEILISEARCH_KEY", ""),
		JWTSecret:          getEnv("JWT_SECRET", "your-secret-key"),
		AdminEmail:         getEnv("ADMIN_EMAIL", ""),
		GitHubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		OAuthRedirectURL:   getEnv("OAUTH_REDIRECT_URL", "http://localhost:8080/api/v1/auth/callback"),
	}
	return AppConfig
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
