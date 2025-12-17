package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	// Server Configuration
	Host string
	Port string

	// Ory Configuration
	OryURL              string // Base URL for Ory (e.g., https://your-project.projects.oryapis.com)
	OryInternalURL      string // Internal URL for server-to-server calls (token exchange)
	OryAdminURL         string // Admin API URL for Ory (for client registration)
	OryClientID         string
	OryClientSecret     string
	OryCallbackURL      string
	OryScopes           string
	OryIntrospectionURL string // URL for token introspection
	OryUserInfoURL      string // URL for user info

	// JWT Configuration (for session management if needed)
	JWTSecret          string
	AccessTokenLifetime  int
	RefreshTokenLifetime int
}

func Load() *Config {
	cfg := &Config{
		Host:                 getEnv("HOST", "0.0.0.0"),
		Port:                 getEnv("PORT", "8080"),
		OryURL:               getEnv("ORY_URL", ""),
		OryInternalURL:       getEnv("ORY_INTERNAL_URL", ""),
		OryAdminURL:          getEnv("ORY_ADMIN_URL", ""),
		OryClientID:          getEnv("ORY_CLIENT_ID", ""),
		OryClientSecret:      getEnv("ORY_CLIENT_SECRET", ""),
		OryCallbackURL:       getEnv("ORY_CALLBACK_URL", "http://localhost:8080/oauth/callback"),
		OryScopes:            getEnv("ORY_SCOPES", "openid offline_access"),
		OryIntrospectionURL:  getEnv("ORY_INTROSPECTION_URL", ""),
		OryUserInfoURL:       getEnv("ORY_USERINFO_URL", ""),
		JWTSecret:            getEnv("JWT_SECRET", "default-secret-change-in-production"),
		AccessTokenLifetime:  getEnvAsInt("ACCESS_TOKEN_LIFETIME", 3600),
		RefreshTokenLifetime: getEnvAsInt("REFRESH_TOKEN_LIFETIME", 604800),
	}

	// Validate required fields
	if cfg.OryURL == "" {
		log.Fatal("ORY_URL is required")
	}
	// Note: ORY_CLIENT_ID and ORY_CLIENT_SECRET are not required
	// MCP clients register themselves dynamically via /oauth/register

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
