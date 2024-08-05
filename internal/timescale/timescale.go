package timescale

import (
	"context"
	"fmt"
	"time"

	_ "embed"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/airgradient"
)

var (
	ErrorSensorIssue = fmt.Errorf("sensor issue - undefined values")
)

type TimescaleClient struct {
	Pool *pgxpool.Pool
}

func NewTimescaleClient(ctx context.Context, connString string) (*TimescaleClient, error) {
	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	cfg.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	err = pool.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &TimescaleClient{Pool: pool}, nil
}

func (c *TimescaleClient) Close() {
	c.Pool.Close()
}

//go:embed queries/insert_airgradient.pgsql
var insertAirgradient string

func (c *TimescaleClient) Insert(ctx context.Context, measure *airgradient.MeasuresCurrentResponse) error {
	// small hack to prevent the documented sensor issues from ending up in the database
	// https://github.com/airgradienthq/arduino/issues/190
	if measure.Rhum < 0 {
		return ErrorSensorIssue
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
		float64((measure.Atmp*9/5)+32), // c to f
		measure.Rhum,
		float64((measure.AtmpCompensated*9/5)+32), // c to f
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
