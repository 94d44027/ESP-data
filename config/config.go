package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	NebulaHost string
	NebulaPort int
	NebulaUser string
	NebulaPwd  string
	Space      string
	AppPort    int
}

// Load reads configuration from environment variables with sensible defaults.
// This satisfies REQ-002: NEBULA_HOST, NEBULA_PORT, NEBULA_USER, NEBULA_PASS, NEBULA_SPACE.
func Load() *Config {
	cfg := &Config{
		// Defaults taken from SRS 2.5.1 GrDB
		NebulaHost: getEnv("NEBULA_HOST", "nebbie.m82"),
		NebulaPort: getEnvInt("NEBULA_PORT", 9669),
		NebulaUser: getEnv("NEBULA_USER", "root"),
		NebulaPwd:  getEnv("NEBULA_PASS", "nebula"),
		Space:      getEnv("NEBULA_SPACE", "ESP01"),
		// App port: main.go currently hardcodes :8080 in ListenAndServe
		AppPort: getEnvInt("APP_PORT", 8080),
	}

	log.Printf("config: Nebula %s:%d space=%s user=%s appPort=%d",
		cfg.NebulaHost, cfg.NebulaPort, cfg.Space, cfg.NebulaUser, cfg.AppPort)

	return cfg
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Printf("config: invalid int for %s=%q, using default %d", key, v, def)
			return def
		}
		return n
	}
	return def
}
