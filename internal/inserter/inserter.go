// Package inserter runs the poll loop: it scrapes each AirGradient monitor on
// the scrape interval, writes the reading, then recomputes and writes the AQI.
package inserter

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/airgradient"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/config"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/timescale"
)

// Runner polls a fixed set of monitors and writes what it finds.
type Runner struct {
	cfg     *config.Config
	ag      *airgradient.Client
	db      *timescale.Client
	metrics *Metrics
}

// NewRunner wires the scraper to the database.
func NewRunner(cfg *config.Config, ag *airgradient.Client, db *timescale.Client, metrics *Metrics) *Runner {
	return &Runner{cfg: cfg, ag: ag, db: db, metrics: metrics}
}

// Run scrapes every instance immediately, then again on each tick of the
// scrape interval, until the context is cancelled.
func (r *Runner) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.cfg.ScrapeInterval)
	defer ticker.Stop()

	for {
		r.scrapeAll(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// scrapeAll polls each instance in turn. One failing monitor does not stop the
// others from being written.
func (r *Runner) scrapeAll(ctx context.Context) {
	for _, host := range r.cfg.AirgradientInstances {
		if ctx.Err() != nil {
			return
		}
		if err := r.scrape(ctx, host); err != nil {
			slog.Error("scrape failed", slog.String("host", host), slog.String("error", err.Error()))
		}
	}
}

// scrape fetches one monitor, writes the reading, then writes its AQI. A
// rejected reading (a faulty sensor) is logged and skipped, but the AQI is
// still recomputed from the history already in the database.
func (r *Runner) scrape(ctx context.Context, host string) error {
	r.metrics.recordScrape(ctx, host)

	measures, err := r.ag.GetCurrentMeasures(ctx, host)
	if err != nil {
		r.metrics.recordScrapeError(ctx, host)
		return err
	}
	serial := measures.Serialno

	err = r.withTimeout(ctx, func(c context.Context) error { return r.db.Insert(c, measures) })
	switch {
	case errors.Is(err, timescale.ErrSensorIssue):
		slog.Warn("skipped reading with sensor issue", slog.String("host", host), slog.String("serial_number", serial))
	case err != nil:
		r.metrics.recordWriteError(ctx, serial)
		slog.Error("could not insert measures", slog.String("serial_number", serial), slog.String("error", err.Error()))
	default:
		r.metrics.recordReading(ctx, measures)
	}

	aqi, err := r.db.CalculateAQI(ctx, serial)
	if err != nil {
		slog.Error("could not calculate AQI", slog.String("serial_number", serial), slog.String("error", err.Error()))
		return nil
	}

	if err := r.withTimeout(ctx, func(c context.Context) error { return r.db.InsertAQI(c, aqi) }); err != nil {
		r.metrics.recordWriteError(ctx, serial)
		slog.Error("could not insert AQI", slog.String("serial_number", serial), slog.String("error", err.Error()))
		return nil
	}
	r.metrics.recordAQI(ctx, aqi)

	slog.Debug("scraped instance",
		slog.String("host", host), slog.String("serial_number", serial),
		slog.Float64("pm02", measures.Pm02), slog.Int64("aqi", aqi.AQI))
	return nil
}

// withTimeout runs a database operation with the insert timeout, detached from
// the parent's cancellation so a shutdown does not abandon a write mid-flight.
func (r *Runner) withTimeout(ctx context.Context, fn func(context.Context) error) error {
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.cfg.InsertTimeout)
	defer cancel()
	return fn(cctx)
}
