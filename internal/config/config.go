package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds all configuration for the proxy server
type Config struct {
	// BindHost is the host/IP to bind the server to
	BindHost string `env:"BIND_HOST" envDefault:"0.0.0.0"`

	// BindPort is the port to listen on
	BindPort int `env:"BIND_PORT" envDefault:"3128"`

	// Timeout is the upstream request timeout
	Timeout time.Duration `env:"TIMEOUT" envDefault:"60s"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return cfg, nil
}

// Address returns the full bind address in format "host:port"
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.BindHost, c.BindPort)
}
