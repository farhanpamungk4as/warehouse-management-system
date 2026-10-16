package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	RedisURL        string
	RedisTTL        time.Duration
	DatabaseURL     string
	HTTPAddr        string
	LogLevel        slog.Level
	DBMaxConns      int32
	StartupTimeout  time.Duration
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration
}

func Load() (Config, error) { return load(os.LookupEnv) }

func load(lookup func(string) (string, bool)) (Config, error) {
	value := func(key, fallback string) string {
		if v, ok := lookup(key); ok {
			return v
		}
		return fallback
	}
	c := Config{RedisURL: value("REDIS_URL", ""), DatabaseURL: value("DATABASE_URL", ""), HTTPAddr: value("HTTP_ADDR", ":8080")}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if _, err := pgxpool.ParseConfig(c.DatabaseURL); err != nil {
		return Config{}, errors.New("DATABASE_URL is invalid")
	}
	host, port, err := net.SplitHostPort(c.HTTPAddr)
	if err != nil || strings.ContainsAny(host, " /\t\n") {
		return Config{}, errors.New("HTTP_ADDR must be host:port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return Config{}, errors.New("HTTP_ADDR port must be between 1 and 65535")
	}
	if err := c.LogLevel.UnmarshalText([]byte(value("LOG_LEVEL", "info"))); err != nil {
		return Config{}, errors.New("LOG_LEVEL is invalid")
	}
	max, err := strconv.ParseInt(value("DB_MAX_CONNS", "10"), 10, 32)
	if err != nil || max < 1 || max > 100 {
		return Config{}, errors.New("DB_MAX_CONNS must be between 1 and 100")
	}
	c.DBMaxConns = int32(max)
	for _, setting := range []struct {
		name, fallback string
		dest           *time.Duration
	}{
		{"REDIS_TTL", "30s", &c.RedisTTL},
		{"STARTUP_TIMEOUT", "10s", &c.StartupTimeout}, {"REQUEST_TIMEOUT", "5s", &c.RequestTimeout}, {"SHUTDOWN_TIMEOUT", "10s", &c.ShutdownTimeout},
	} {
		d, err := time.ParseDuration(value(setting.name, setting.fallback))
		if err != nil || d <= 0 || d > time.Minute {
			return Config{}, fmt.Errorf("%s must be a positive duration no greater than 1m", setting.name)
		}
		*setting.dest = d
	}
	return c, nil
}
