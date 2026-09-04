// Package timescale writes AirGradient readings and the derived AQI into
// TimescaleDB, and reads the trailing-day averages the AQI is computed from.
package timescale

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/airgradient"
)

// ErrSensorIssue is returned when a reading carries the impossible values the
// firmware emits while a sensor is faulty, so they never reach the database.
// See https://github.com/airgradienthq/arduino/issues/190.
var ErrSensorIssue = errors.New("sensor issue - undefined values")

// Client is a TimescaleDB connection pool.
type Client struct {
	Pool *pgxpool.Pool
}

// NewClient opens a connection pool and pings the database.
func NewClient(ctx context.Context, connString string) (*Client, error) {
	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Client{Pool: pool}, nil
}

// Close releases the pool.
func (c *Client) Close() { c.Pool.Close() }

//go:embed queries/insert_airgradient.pgsql
var insertAirgradient string

// Insert writes one reading, stamped with the current time. The temperatures are
// converted from the firmware's Celsius to Fahrenheit to match the table.
func (c *Client) Insert(ctx context.Context, measure *airgradient.MeasuresCurrentResponse) error {
	if measure.Rhum < 0 {
		return ErrSensorIssue
	}

	_, err := c.Pool.Exec(ctx, insertAirgradient,
		time.Now(),
		measure.Wifi,
		measure.Serialno,
		measure.Rco2,
		measure.Pm01,
		measure.Pm02,
		measure.Pm10,
		measure.Pm003Count,
		celsiusToFahrenheit(measure.Atmp),
		measure.Rhum,
		celsiusToFahrenheit(measure.AtmpCompensated),
		measure.RhumCompensated,
		measure.TvocIndex,
		measure.TvocRaw,
		measure.NoxIndex,
		measure.NoxRaw,
		measure.Boot,
		measure.BootCount,
		measure.Firmware,
		measure.Model,
	)
	if err != nil {
		return fmt.Errorf("insert data: %w", err)
	}

	return nil
}

func celsiusToFahrenheit(c float64) float64 {
	return c*9/5 + 32
}
