package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"hacktrapagent-api-endpoint/internal/config"
	"hacktrapagent-api-endpoint/internal/model"
)

type Store struct {
	conn  clickhouse.Conn
	table string
}

func NewStore(ctx context.Context, cfg config.ClickHouseConfig) (*Store, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: cfg.Addrs,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		DialTimeout:     cfg.DialTimeout,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		Protocol:        clickhouse.Native,
		TLS:             tlsConfig(cfg.Secure),
	})
	if err != nil {
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}

	return &Store{
		conn:  conn,
		table: cfg.Table,
	}, nil
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	query := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
	event_datetime DateTime,
	registered_at DateTime,
	source String,
	mashine_id String,
	container_id Nullable(String),
	unit_name Nullable(String),
	hostname Nullable(String),
	id Nullable(String),
	dst_ip Nullable(String),
	dst_fqdn Nullable(String),
	src_ip String,
	src_port Nullable(UInt16),
	dst_port Nullable(UInt16),
	protocol Nullable(String),
	service_port Nullable(UInt16),
	action String,
	extra Nullable(String)
) ENGINE = MergeTree
ORDER BY (registered_at, mashine_id, source)
`, s.table)

	return s.conn.Exec(ctx, query)
}

func (s *Store) InsertEvent(ctx context.Context, event model.EventRecord) error {
	query := fmt.Sprintf(`
INSERT INTO %s (
	event_datetime, registered_at, source, mashine_id, container_id, unit_name, hostname, id, dst_ip, dst_fqdn, src_ip, src_port, dst_port, protocol, service_port, action, extra
) VALUES
`, s.table)

	return s.conn.Exec(
		ctx,
		query,
		event.EventDatetime,
		event.RegisteredAt,
		event.Source,
		event.MashineID,
		event.ContainerID,
		event.UnitName,
		event.Hostname,
		event.ID,
		event.DstIP,
		event.DstFQDN,
		event.SrcIP,
		event.SrcPort,
		event.DstPort,
		event.Protocol,
		event.ServicePort,
		event.Action,
		event.Extra,
	)
}

func (s *Store) LoadRecent(ctx context.Context, since time.Time) ([]model.EventRecord, error) {
	query := fmt.Sprintf(`
SELECT
	event_datetime, registered_at, source, mashine_id, container_id, unit_name, hostname, id, dst_ip, dst_fqdn, src_ip, src_port, dst_port, protocol, service_port, action, extra
FROM %s
WHERE registered_at >= ?
ORDER BY registered_at ASC
`, s.table)

	rows, err := s.conn.Query(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]model.EventRecord, 0)
	for rows.Next() {
		var event model.EventRecord
		if err := rows.Scan(
			&event.EventDatetime,
			&event.RegisteredAt,
			&event.Source,
			&event.MashineID,
			&event.ContainerID,
			&event.UnitName,
			&event.Hostname,
			&event.ID,
			&event.DstIP,
			&event.DstFQDN,
			&event.SrcIP,
			&event.SrcPort,
			&event.DstPort,
			&event.Protocol,
			&event.ServicePort,
			&event.Action,
			&event.Extra,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

func (s *Store) Close() error {
	return s.conn.Close()
}
