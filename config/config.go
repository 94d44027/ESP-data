package config

type Config struct {
	NebulaHost string
	NebulaPort int
	NebulaUser string
	NebulaPwd  string
	Space      string
	AppPort    int
}

func Load() *Config {
	// read from env vars or defaults
}
