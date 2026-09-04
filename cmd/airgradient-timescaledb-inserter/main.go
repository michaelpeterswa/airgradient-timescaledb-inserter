package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"alpineworks.io/ootel"
	"github.com/urfave/cli/v3"
	"go.opentelemetry.io/contrib/instrumentation/host"
	"go.opentelemetry.io/contrib/instrumentation/runtime"

	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/airgradient"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/config"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/inserter"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/logging"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/schema"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/timescale"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/version"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cmd := &cli.Command{
		Name:    "airgradient-timescaledb-inserter",
		Usage:   "poll AirGradient monitors and write their readings into TimescaleDB",
		Version: version.Version,
		Before:  setupLogging,
		// With no subcommand the inserter runs, so the container's bare
		// ENTRYPOINT keeps the behaviour it had before there were subcommands.
		Action: runInserter,
		Commands: []*cli.Command{
			{
				Name:   "run",
				Usage:  "poll the monitors and write readings on the scrape interval (the default)",
				Action: runInserter,
			},
			{
				Name:  "migrate",
				Usage: "manage the database schema with the embedded migrations",
				Commands: []*cli.Command{
					{
						Name:   "up",
						Usage:  "apply every pending migration; the init container command",
						Action: migrateUp,
					},
					{
						Name:   "down",
						Usage:  "roll back the most recent migration (one step; drops that table)",
						Action: migrateDown,
					},
					{
						Name:   "version",
						Usage:  "print the current schema version and whether it is dirty",
						Action: migrateVersion,
					},
					{
						Name:      "force",
						Usage:     "mark the schema as being at a version, without running anything",
						ArgsUsage: "<version>",
						Action:    migrateForce,
					},
				},
			},
			{
				Name:   "version",
				Usage:  "print the build version, commit, and date",
				Action: printVersion,
			},
		},
	}

	if err := cmd.Run(ctx, os.Args); err != nil {
		slog.Error("fatal", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// setupLogging installs the JSON handler at LOG_LEVEL before any command runs.
// It reads the environment directly so a bad config still logs its error.
func setupLogging(ctx context.Context, _ *cli.Command) (context.Context, error) {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "error"
	}

	slogLevel, err := logging.LogLevelToSlogLevel(logLevel)
	if err != nil {
		return ctx, fmt.Errorf("could not convert log level: %w", err)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slogLevel,
	})))
	return ctx, nil
}

func runInserter(ctx context.Context, _ *cli.Command) error {
	c, err := config.NewConfig()
	if err != nil {
		return fmt.Errorf("could not create config: %w", err)
	}

	exporterType := ootel.ExporterTypePrometheus
	if c.Local {
		exporterType = ootel.ExporterTypeOTLPGRPC
	}

	ootelClient := ootel.NewOotelClient(
		ootel.WithMetricConfig(
			ootel.NewMetricConfig(
				c.MetricsEnabled,
				exporterType,
				c.MetricsPort,
			),
		),
		ootel.WithTraceConfig(
			ootel.NewTraceConfig(
				c.TracingEnabled,
				c.TracingSampleRate,
				c.TracingService,
				version.Version,
			),
		),
	)

	shutdown, err := ootelClient.Init(ctx)
	if err != nil {
		return fmt.Errorf("could not create ootel client: %w", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(5 * time.Second)); err != nil {
		return fmt.Errorf("could not create runtime metrics: %w", err)
	}
	if err := host.Start(); err != nil {
		return fmt.Errorf("could not create host metrics: %w", err)
	}

	timescaleClient, err := timescale.NewClient(ctx, c.TimescaleConnString)
	if err != nil {
		return fmt.Errorf("could not create timescale client: %w", err)
	}
	defer timescaleClient.Close()

	airgradientClient := airgradient.NewClient(&http.Client{
		Timeout: c.ScrapeTimeout,
	})

	metrics, err := inserter.NewMetrics()
	if err != nil {
		return fmt.Errorf("could not create metrics: %w", err)
	}

	runner := inserter.NewRunner(c, airgradientClient, timescaleClient, metrics)

	slog.Info("airgradient timescaledb inserter started",
		slog.String("version", version.Version),
		slog.String("commit", version.Commit),
		slog.Any("instances", c.AirgradientInstances),
		slog.Duration("scrape_interval", c.ScrapeInterval))

	if err := runner.Run(ctx); err != nil {
		return fmt.Errorf("runner stopped with an error: %w", err)
	}

	slog.Info("shutdown complete")
	return nil
}

// openMigrator builds a migrator from TIMESCALE_CONN_STRING.
func openMigrator() (*schema.Migrator, error) {
	c, err := config.NewMigrateConfig()
	if err != nil {
		return nil, err
	}
	return schema.New(c.TimescaleConnString)
}

func migrateUp(_ context.Context, _ *cli.Command) error {
	mg, err := openMigrator()
	if err != nil {
		return err
	}
	defer func() { _ = mg.Close() }()

	applied, err := mg.Up()
	if err != nil {
		return err
	}
	v, dirty, err := mg.Version()
	if err != nil {
		return err
	}
	slog.Info("migrations applied",
		slog.Bool("changed", applied), slog.Uint64("version", uint64(v)), slog.Bool("dirty", dirty))
	fmt.Printf("schema at version %d (changed=%t, dirty=%t)\n", v, applied, dirty)
	return nil
}

func migrateDown(_ context.Context, _ *cli.Command) error {
	mg, err := openMigrator()
	if err != nil {
		return err
	}
	defer func() { _ = mg.Close() }()

	if err := mg.Down(); err != nil {
		return err
	}
	v, dirty, err := mg.Version()
	if err != nil {
		return err
	}
	fmt.Printf("schema at version %d (dirty=%t)\n", v, dirty)
	return nil
}

func migrateVersion(_ context.Context, _ *cli.Command) error {
	mg, err := openMigrator()
	if err != nil {
		return err
	}
	defer func() { _ = mg.Close() }()

	v, dirty, err := mg.Version()
	if err != nil {
		return err
	}
	fmt.Printf("schema at version %d (dirty=%t)\n", v, dirty)
	if dirty {
		return fmt.Errorf("schema is dirty; repair the database then run `migrate force %d`", v)
	}
	return nil
}

func migrateForce(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("usage: migrate force <version>")
	}
	v, err := strconv.Atoi(cmd.Args().First())
	if err != nil {
		return fmt.Errorf("parse version %q: %w", cmd.Args().First(), err)
	}

	mg, err := openMigrator()
	if err != nil {
		return err
	}
	defer func() { _ = mg.Close() }()

	if err := mg.Force(v); err != nil {
		return err
	}
	fmt.Printf("schema forced to version %d\n", v)
	return nil
}

func printVersion(_ context.Context, _ *cli.Command) error {
	fmt.Printf("airgradient-timescaledb-inserter %s (commit %s, built %s)\n",
		version.Version, version.Commit, version.Date)
	return nil
}
