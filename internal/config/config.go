// Package config loads application configuration (database credentials and
// other server settings) from environment variables. A .env file in the
// working directory is loaded first, if present, purely as a local-dev
// convenience — real deployments should set the environment directly.
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

const (
	dbURLEnvVar    = "GATOR_DB_URL"
	redisURLEnvVar = "GATOR_REDIS_URL"
)

type Config struct {
	// DBURL is the Postgres connection string, e.g.
	// postgres://user:password@localhost:5432/gator?sslmode=disable
	DBURL string
	// RedisURL is an optional Redis connection string, e.g.
	// redis://localhost:6379/0. Empty means caching is disabled — unlike
	// DBURL, this is not required.
	RedisURL string
}

// Load reads configuration from the environment. It returns an error if a
// required variable is missing.
func Load() (*Config, error) {
	_ = godotenv.Load()

	dbURL := os.Getenv(dbURLEnvVar)
	if dbURL == "" {
		return nil, fmt.Errorf("%s environment variable is not set", dbURLEnvVar)
	}

	return &Config{
		DBURL:    dbURL,
		RedisURL: os.Getenv(redisURLEnvVar),
	}, nil
}
