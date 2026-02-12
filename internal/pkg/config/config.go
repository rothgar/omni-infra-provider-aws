package config

import (
	"os"
)

type Config struct {
	AWS struct {
		Region string `yaml:"region"`
	} `yaml:"aws"`
}

func LoadConfig(path string) (*Config, error) {
	// For now, we'll just return a default config or load from file if it exists
	cfg := &Config{}
	cfg.AWS.Region = os.Getenv("AWS_REGION")
	if cfg.AWS.Region == "" {
		cfg.AWS.Region = os.Getenv("AWS_DEFAULT_REGION")
	}
	// We could add file loading logic here later
	return cfg, nil
}
