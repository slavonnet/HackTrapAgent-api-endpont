package clickhouse

import "crypto/tls"

func tlsConfig(enabled bool) *tls.Config {
	if !enabled {
		return nil
	}

	return &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
}
