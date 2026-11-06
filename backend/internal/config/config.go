package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	DefaultAddr                        = ":8080"
	DefaultOpenBiometricsBaseURL       = "http://openbiometrics:8000"
	DefaultMaxImageBytes         int64 = 5 << 20
	MaxAllowedImageBytes         int64 = 25 << 20
	DefaultBiometricsTimeout           = 30 * time.Second
	MaxBiometricsTimeout               = 10 * time.Minute
)

type Config struct {
	Addr                  string
	OpenBiometricsBaseURL string
	MaxImageBytes         int64
	BiometricsTimeout     time.Duration
}

func Load() (Config, error) {
	addr := envOrDefault("GO_ADDR", DefaultAddr)
	if err := validateAddr(addr); err != nil {
		return Config{}, err
	}

	maxImageBytes, err := positiveInt64Env("MAX_IMAGE_BYTES", DefaultMaxImageBytes)
	if err != nil {
		return Config{}, err
	}
	if maxImageBytes > MaxAllowedImageBytes {
		return Config{}, fmt.Errorf("MAX_IMAGE_BYTES must not exceed %d", MaxAllowedImageBytes)
	}

	timeoutSeconds, err := positiveInt64Env(
		"BIOMETRICS_TIMEOUT_SECONDS",
		int64(DefaultBiometricsTimeout/time.Second),
	)
	if err != nil {
		return Config{}, err
	}
	maxTimeoutSeconds := int64(MaxBiometricsTimeout / time.Second)
	if timeoutSeconds > maxTimeoutSeconds {
		return Config{}, fmt.Errorf(
			"BIOMETRICS_TIMEOUT_SECONDS must not exceed %d",
			maxTimeoutSeconds,
		)
	}
	timeout := time.Duration(timeoutSeconds) * time.Second

	return Config{
		Addr:                  addr,
		OpenBiometricsBaseURL: envOrDefault("OPENBIOMETRICS_BASE_URL", DefaultOpenBiometricsBaseURL),
		MaxImageBytes:         maxImageBytes,
		BiometricsTimeout:     timeout,
	}, nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func positiveInt64Env(name string, fallback int64) (int64, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func validateAddr(addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("GO_ADDR must be a host:port address")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("GO_ADDR port must be between 1 and 65535")
	}
	return nil
}
