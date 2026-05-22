package config

import "testing"

func TestLoadFromEnvParsesJSONSettings(t *testing.T) {
	t.Setenv("LIMITS", `[{"keys":["source","mashine_id"],"window":60,"limit":100}]`)
	t.Setenv("BLACKLIST", `[{"source":"10.0.0.1"}]`)
	t.Setenv("WHITELIST", `[{"mashine_id":"trusted-host"}]`)
	t.Setenv("CLICKHOUSE_ADDRS", "ch1:9000,ch2:9000")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if len(cfg.Limits) != 1 {
		t.Fatalf("limits len = %d, want 1", len(cfg.Limits))
	}
	if len(cfg.Blacklist) != 1 {
		t.Fatalf("blacklist len = %d, want 1", len(cfg.Blacklist))
	}
	if len(cfg.Whitelist) != 1 {
		t.Fatalf("whitelist len = %d, want 1", len(cfg.Whitelist))
	}
	if len(cfg.ClickHouse.Addrs) != 2 {
		t.Fatalf("clickhouse addrs len = %d, want 2", len(cfg.ClickHouse.Addrs))
	}
}
