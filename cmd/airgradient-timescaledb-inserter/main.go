package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alpineworks/ootel"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/airgradient"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/config"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/logging"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/timescale"
)

func main() {
	slogHandler := slog.NewJSONHandler(os.Stdout, nil)
	slog.SetDefault(slog.New(slogHandler))

	slog.Info("welcome to airgradient-timescaledb-inserter!")

	c, err := config.NewConfig()
	if err != nil {
		slog.Error("could not create config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	slogLevel, err := logging.LogLevelToSlogLevel(c.String(config.LogLevel))
	if err != nil {
		slog.Error("could not parse log level", slog.String("error", err.Error()))
		os.Exit(1)
	}

	slog.SetLogLoggerLevel(slogLevel)

	ctx := context.Background()

	ootelClient := ootel.NewOotelClient(
		ootel.WithMetricConfig(
			ootel.NewMetricConfig(
				c.Bool(config.MetricsEnabled),
				c.Int(config.MetricsPort),
			),
		),
		ootel.WithTraceConfig(
			ootel.NewTraceConfig(
				c.Bool(config.TracingEnabled),
				c.Float64(config.TracingSampleRate),
				c.String(config.TracingService),
				c.String(config.TracingVersion),
			),
		),
	)

	shutdown, err := ootelClient.Init(ctx)
	if err != nil {
		panic(err)
	}

	defer func() {
		_ = shutdown(ctx)
	}()

	timescaleClient, err := timescale.NewTimescaleClient(ctx, c.String(config.TimescaleConnString))
	if err != nil {
		slog.Error("could not create timescale client", slog.String("error", err.Error()))
		os.Exit(1)
	}

	airgradientClient := airgradient.NewAirgradientClient(&http.Client{
		Timeout: time.Second * 5,
	})

	airgradientInstances := strings.Split(c.String(config.AirgradientInstances), ",")

	scrapeInterval := c.Duration(config.ScrapeInterval)
	scrapeTicker := time.NewTicker(scrapeInterval)
	defer scrapeTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-scrapeTicker.C:
			for _, clientURL := range airgradientInstances {
				measures, err := airgradientClient.GetCurrentMeasures(clientURL)
				if err != nil {
					slog.Error("could not get measures", slog.String("error", err.Error()))
					continue
				}
				err = timescaleClient.Insert(ctx, measures)
				if err != nil {
					slog.Error("could not insert measures", slog.String("error", err.Error()))
				}
			}
		}
	}
}
