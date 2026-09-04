package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"alpineworks.io/ootel"
	"go.opentelemetry.io/contrib/instrumentation/host"
	"go.opentelemetry.io/contrib/instrumentation/runtime"

	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/airgradient"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/config"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/inserter"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/logging"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/timescale"
)

func main() {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "error"
	}

	slogLevel, err := logging.LogLevelToSlogLevel(logLevel)
	if err != nil {
		log.Fatalf("could not convert log level: %s", err)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slogLevel,
	})))

	c, err := config.NewConfig()
	if err != nil {
		slog.Error("could not create config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
				c.TracingVersion,
			),
		),
	)

	shutdown, err := ootelClient.Init(ctx)
	if err != nil {
		slog.Error("could not create ootel client", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() { _ = shutdown(context.Background()) }()

	if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(5 * time.Second)); err != nil {
		slog.Error("could not create runtime metrics", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := host.Start(); err != nil {
		slog.Error("could not create host metrics", slog.String("error", err.Error()))
		os.Exit(1)
	}

	timescaleClient, err := timescale.NewClient(ctx, c.TimescaleConnString)
	if err != nil {
		slog.Error("could not create timescale client", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer timescaleClient.Close()

	airgradientClient := airgradient.NewClient(&http.Client{
		Timeout: c.ScrapeTimeout,
	})

	metrics, err := inserter.NewMetrics()
	if err != nil {
		slog.Error("could not create metrics", slog.String("error", err.Error()))
		os.Exit(1)
	}

	runner := inserter.NewRunner(c, airgradientClient, timescaleClient, metrics)

	slog.Info("airgradient timescaledb inserter started",
		slog.Any("instances", c.AirgradientInstances),
		slog.Duration("scrape_interval", c.ScrapeInterval))

	if err := runner.Run(ctx); err != nil {
		slog.Error("runner stopped with an error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	slog.Info("shutdown complete")
}
