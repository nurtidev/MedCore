package database

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type ClickHouseConfig struct {
	DSN          string
	MaxOpenConns int
	DialTimeout  time.Duration
}

// NewClickHouseConn создаёт соединение с ClickHouse.
func NewClickHouseConn(ctx context.Context, cfg ClickHouseConfig) (driver.Conn, error) {
	opts, err := clickhouse.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("database.NewClickHouseConn: parse dsn: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		opts.MaxOpenConns = cfg.MaxOpenConns
	}
	if cfg.DialTimeout > 0 {
		opts.DialTimeout = cfg.DialTimeout
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("database.NewClickHouseConn: open: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database.NewClickHouseConn: ping: %w", err)
	}

	return conn, nil
}
