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

	// TTB calculation parameters (ALG-REQ-071, ALG-REQ-072, ALG-REQ-075)
	OrientationTime   float64 // hours; default 0.25 (15 min). ALG-REQ-071.
	SwitchoverTime    float64 // hours; default 0.1667 (10 min). ALG-REQ-072.
	PriorityTolerance int     // levels below top; default 1. ALG-REQ-075.

	// MariaDB (RDBMS) parameters (ADR-REQ-002)
	MariaHost    string
	MariaPort    int
	MariaUser    string
	MariaPass    string
	MariaDB      string
	MariaEnabled bool
}

// Load reads configuration from environment variables with sensible defaults.
// This satisfies REQ-002: NEBULA_HOST, NEBULA_PORT, NEBULA_USER, NEBULA_PASS, NEBULA_SPACE.
// MariaDB parameters added per ADR-REQ-002.
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

		// TTB calculation defaults (ALG-REQ-071, ALG-REQ-072, ALG-REQ-075)
		OrientationTime:   getEnvFloat("TTB_ORIENTATION_TIME", 0.25),
		SwitchoverTime:    getEnvFloat("TTB_SWITCHOVER_TIME", 0.1667),
		PriorityTolerance: getEnvInt("TTB_PRIORITY_TOLERANCE", 1),

		// MariaDB defaults (ADR-REQ-002)
		MariaHost:    getEnv("MARIA_HOST", "nebbie.m82"),
		MariaPort:    getEnvInt("MARIA_PORT", 3306),
		MariaUser:    getEnv("MARIA_USER", "esp01"),
		MariaPass:    getEnv("MARIA_PASS", "nebula1"),
		MariaDB:      getEnv("MARIA_DB", "ESP01"),
		MariaEnabled: getEnvBool("MARIA_ENABLED", false),
	}

	log.Printf("config: Nebula %s:%d space=%s user=%s appPort=%d",
		cfg.NebulaHost, cfg.NebulaPort, cfg.Space, cfg.NebulaUser, cfg.AppPort)
	log.Printf("config: TTB params — orientationTime=%.4fh switchoverTime=%.4fh priorityTolerance=%d",
		cfg.OrientationTime, cfg.SwitchoverTime, cfg.PriorityTolerance)
	log.Printf("config: MariaDB enabled=%v host=%s:%d db=%s",
		cfg.MariaEnabled, cfg.MariaHost, cfg.MariaPort, cfg.MariaDB)

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

func getEnvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			log.Printf("config: invalid float for %s=%q, using default %.4f", key, v, def)
			return def
		}
		return f
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			log.Printf("config: invalid bool for %s=%q, using default %v", key, v, def)
			return def
		}
		return b
	}
	return def
}
