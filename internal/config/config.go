package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"hacktrapagent-api-endpoint/internal/filter"
	"hacktrapagent-api-endpoint/internal/ratelimit"
)

const (
	defaultHTTPAddr        = ":8080"
	defaultRequestBodySize = 1024 * 1024
)

type Config struct {
	HTTPAddr              string
	RequestBodyLimitBytes int64
	ClickHouse            ClickHouseConfig
	Limits                []ratelimit.Rule
	Blacklist             []filter.Rule
	Whitelist             []filter.Rule
}

type ClickHouseConfig struct {
	Addrs           []string
	Database        string
	Username        string
	Password        string
	Table           string
	Secure          bool
	DialTimeout     time.Duration
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func LoadFromEnv() (Config, error) {
	cfg := Config{
		HTTPAddr:              getEnv("HTTP_ADDR", defaultHTTPAddr),
		RequestBodyLimitBytes: int64(getEnvInt("REQUEST_BODY_LIMIT_BYTES", defaultRequestBodySize)),
		ClickHouse: ClickHouseConfig{
			Addrs:           splitCSV(getEnv("CLICKHOUSE_ADDRS", "localhost:9000")),
			Database:        getEnv("CLICKHOUSE_DATABASE", "default"),
			Username:        getEnv("CLICKHOUSE_USERNAME", "default"),
			Password:        os.Getenv("CLICKHOUSE_PASSWORD"),
			Table:           getEnv("CLICKHOUSE_TABLE", "access_events"),
			Secure:          getEnvBool("CLICKHOUSE_SECURE", false),
			DialTimeout:     time.Duration(getEnvInt("CLICKHOUSE_DIAL_TIMEOUT_SECONDS", 5)) * time.Second,
			MaxOpenConns:    getEnvInt("CLICKHOUSE_MAX_OPEN_CONNS", 10),
			MaxIdleConns:    getEnvInt("CLICKHOUSE_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: time.Duration(getEnvInt("CLICKHOUSE_CONN_MAX_LIFETIME_SECONDS", 300)) * time.Second,
		},
	}

	if cfg.ClickHouse.Table == "" {
		return Config{}, fmt.Errorf("CLICKHOUSE_TABLE must not be empty")
	}
	if len(cfg.ClickHouse.Addrs) == 0 {
		return Config{}, fmt.Errorf("CLICKHOUSE_ADDRS must not be empty")
	}

	limitsRaw := strings.TrimSpace(os.Getenv("LIMITS"))
	if limitsRaw != "" {
		if err := json.Unmarshal([]byte(limitsRaw), &cfg.Limits); err != nil {
			return Config{}, fmt.Errorf("failed to parse LIMITS: %w", err)
		}
	}

	var err error
	cfg.Blacklist, err = parseFilterRules("BLACKLIST")
	if err != nil {
		return Config{}, err
	}
	cfg.Whitelist, err = parseFilterRules("WHITELIST")
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func parseFilterRules(envName string) ([]filter.Rule, error) {
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return nil, nil
	}

	var rules []filter.Rule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", envName, err)
	}
	return rules, nil
}

func getEnv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
