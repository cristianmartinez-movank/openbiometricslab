package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("GO_ADDR", "")
	t.Setenv("OPENBIOMETRICS_BASE_URL", "")
	t.Setenv("MAX_IMAGE_BYTES", "")
	t.Setenv("BIOMETRICS_TIMEOUT_SECONDS", "")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Addr != DefaultAddr {
		t.Errorf("Addr = %q, want %q", got.Addr, DefaultAddr)
	}
	if got.OpenBiometricsBaseURL != DefaultOpenBiometricsBaseURL {
		t.Errorf("OpenBiometricsBaseURL = %q, want default", got.OpenBiometricsBaseURL)
	}
	if got.MaxImageBytes != DefaultMaxImageBytes {
		t.Errorf("MaxImageBytes = %d, want %d", got.MaxImageBytes, DefaultMaxImageBytes)
	}
	if got.BiometricsTimeout != DefaultBiometricsTimeout {
		t.Errorf("BiometricsTimeout = %s, want %s", got.BiometricsTimeout, DefaultBiometricsTimeout)
	}
}

func TestLoadValidOverrides(t *testing.T) {
	t.Setenv("GO_ADDR", "127.0.0.1:9090")
	t.Setenv("OPENBIOMETRICS_BASE_URL", "http://engine.local:8000")
	t.Setenv("MAX_IMAGE_BYTES", "1048576")
	t.Setenv("BIOMETRICS_TIMEOUT_SECONDS", "45")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Addr != "127.0.0.1:9090" {
		t.Errorf("Addr = %q, want 127.0.0.1:9090", got.Addr)
	}
	if got.OpenBiometricsBaseURL != "http://engine.local:8000" {
		t.Errorf("OpenBiometricsBaseURL = %q, want override", got.OpenBiometricsBaseURL)
	}
	if got.MaxImageBytes != 1<<20 {
		t.Errorf("MaxImageBytes = %d, want %d", got.MaxImageBytes, 1<<20)
	}
	if got.BiometricsTimeout != 45*time.Second {
		t.Errorf("BiometricsTimeout = %s, want 45s", got.BiometricsTimeout)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "address without port", key: "GO_ADDR", value: "localhost"},
		{name: "invalid address port", key: "GO_ADDR", value: ":abc"},
		{name: "port out of range", key: "GO_ADDR", value: ":65536"},
		{name: "zero image size", key: "MAX_IMAGE_BYTES", value: "0"},
		{name: "invalid image size", key: "MAX_IMAGE_BYTES", value: "large"},
		{name: "image size exceeds bound", key: "MAX_IMAGE_BYTES", value: "26214401"},
		{name: "zero timeout", key: "BIOMETRICS_TIMEOUT_SECONDS", value: "0"},
		{name: "invalid timeout", key: "BIOMETRICS_TIMEOUT_SECONDS", value: "soon"},
		{name: "timeout exceeds bound", key: "BIOMETRICS_TIMEOUT_SECONDS", value: "601"},
		{name: "timeout overflow", key: "BIOMETRICS_TIMEOUT_SECONDS", value: "9223372036854775807"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("GO_ADDR", "")
			t.Setenv("MAX_IMAGE_BYTES", "")
			t.Setenv("BIOMETRICS_TIMEOUT_SECONDS", "")
			t.Setenv(test.key, test.value)

			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want configuration validation error")
			}
		})
	}
}
