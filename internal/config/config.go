package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	LogLevel string

	HTTPAddr          string
	ShutdownTimeout   time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration

	DatabaseURL     string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
	QueryTimeout    time.Duration
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	var errs []error

	cfg := Config{
		HTTPAddr:          getString("HTTP_ADDR", ":8080"),
		ShutdownTimeout:   getDuration("SHUTDOWN_TIMEOUT", 5*time.Second, &errs),
		ReadHeaderTimeout: getDuration("HTTP_READ_HEADER_TIMEOUT", 2*time.Second, &errs),
		ReadTimeout:       getDuration("HTTP_READ_TIMEOUT", 10*time.Second, &errs),
		WriteTimeout:      getDuration("HTTP_WRITE_TIMEOUT", 10*time.Second, &errs),
		IdleTimeout:       getDuration("HTTP_IDLE_TIMEOUT", 3*time.Minute, &errs),

		DatabaseURL:     os.Getenv("DATABASE_URL"),
		MaxConns:        getInt32("DATABASE_MAX_CONNS", 10, &errs),
		MinConns:        getInt32("DATABASE_MIN_CONNS", 2, &errs),
		MaxConnLifetime: getDuration("DATABASE_MAX_CONN_LIFETIME", 30*time.Minute, &errs),
		ConnectTimeout:  getDuration("DATABASE_CONNECT_TIMEOUT", 5*time.Second, &errs),
		QueryTimeout:    getDuration("DATABASE_QUERY_TIMEOUT", 3*time.Second, &errs),
	}

	cfg.LogLevel = getString("LOG_LEVEL", "info")
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("LOG_LEVEL: must be debug, info, warn or error, got %q", cfg.LogLevel))
	}

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if cfg.MaxConns <= 0 {
		errs = append(errs, errors.New("DATABASE_MAX_CONNS must be positive"))
	}
	if cfg.MinConns < 0 || cfg.MinConns > cfg.MaxConns {
		errs = append(errs, fmt.Errorf("DATABASE_MIN_CONNS must be between 0 and DATABASE_MAX_CONNS (%d)", cfg.MaxConns))
	}
	if cfg.QueryTimeout >= cfg.WriteTimeout {
		errs = append(errs, errors.New("DATABASE_QUERY_TIMEOUT must be less than HTTP_WRITE_TIMEOUT"))
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

func getString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration, errs *[]error) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s: invalid duration %q", key, v))
		return def
	}
	if d <= 0 {
		*errs = append(*errs, fmt.Errorf("%s: must be positive, got %s", key, d))
		return def
	}
	return d
}

func getInt32(key string, def int32, errs *[]error) int32 {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s: invalid integer %q", key, v))
		return def
	}
	return int32(n)
}
