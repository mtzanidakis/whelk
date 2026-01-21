package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear environment variables
	os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Check defaults
	if cfg.BindHost != "0.0.0.0" {
		t.Errorf("expected BindHost=0.0.0.0, got %s", cfg.BindHost)
	}
	if cfg.BindPort != 3128 {
		t.Errorf("expected BindPort=3128, got %d", cfg.BindPort)
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("expected Timeout=60s, got %v", cfg.Timeout)
	}
}

func TestLoad_CustomValues(t *testing.T) {
	// Set environment variables
	os.Setenv("BIND_HOST", "127.0.0.1")
	os.Setenv("BIND_PORT", "8080")
	os.Setenv("TIMEOUT", "30s")
	defer os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Check custom values
	if cfg.BindHost != "127.0.0.1" {
		t.Errorf("expected BindHost=127.0.0.1, got %s", cfg.BindHost)
	}
	if cfg.BindPort != 8080 {
		t.Errorf("expected BindPort=8080, got %d", cfg.BindPort)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected Timeout=30s, got %v", cfg.Timeout)
	}
}

func TestLoad_InvalidTimeout(t *testing.T) {
	os.Setenv("TIMEOUT", "invalid")
	defer os.Clearenv()

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid timeout, got nil")
	}
}

func TestAddress(t *testing.T) {
	cfg := &Config{
		BindHost: "192.168.1.1",
		BindPort: 9090,
	}

	expected := "192.168.1.1:9090"
	if addr := cfg.Address(); addr != expected {
		t.Errorf("expected Address()=%s, got %s", expected, addr)
	}
}
