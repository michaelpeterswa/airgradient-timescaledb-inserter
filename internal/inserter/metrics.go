package inserter

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/airgradient"
	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/timescale"
)

const meterName = "github.com/michaelpeterswa/airgradient-timescaledb-inserter"

// Metrics reports the health of the poll loop and the latest readings. The
// scrape counters are labelled by host, because a failed scrape has no serial
// number yet; everything else is labelled by serial_number.
type Metrics struct {
	scrapes      metric.Int64Counter
	scrapeErrors metric.Int64Counter
	writeErrors  metric.Int64Counter

	pm02 metric.Float64Gauge
	pm10 metric.Float64Gauge
	rco2 metric.Float64Gauge
	aqi  metric.Int64Gauge
}

// NewMetrics registers the instruments.
func NewMetrics() (*Metrics, error) {
	meter := otel.Meter(meterName)
	m := &Metrics{}
	var err error

	if m.scrapes, err = meter.Int64Counter("airgradient.scrapes",
		metric.WithDescription("Scrapes attempted against a monitor")); err != nil {
		return nil, err
	}
	if m.scrapeErrors, err = meter.Int64Counter("airgradient.scrape_errors",
		metric.WithDescription("Scrapes that failed to return a reading")); err != nil {
		return nil, err
	}
	if m.writeErrors, err = meter.Int64Counter("airgradient.write_errors",
		metric.WithDescription("Failed writes to the database")); err != nil {
		return nil, err
	}
	if m.pm02, err = meter.Float64Gauge("airgradient.pm02",
		metric.WithDescription("Latest PM2.5 reading (ug/m3)")); err != nil {
		return nil, err
	}
	if m.pm10, err = meter.Float64Gauge("airgradient.pm10",
		metric.WithDescription("Latest PM10 reading (ug/m3)")); err != nil {
		return nil, err
	}
	if m.rco2, err = meter.Float64Gauge("airgradient.rco2",
		metric.WithDescription("Latest CO2 reading (ppm)")); err != nil {
		return nil, err
	}
	if m.aqi, err = meter.Int64Gauge("airgradient.aqi",
		metric.WithDescription("Latest US EPA AQI from the trailing 24 hours")); err != nil {
		return nil, err
	}
	return m, nil
}

// recordScrape counts one scrape attempt.
func (m *Metrics) recordScrape(ctx context.Context, host string) {
	if m == nil {
		return
	}
	m.scrapes.Add(ctx, 1, metric.WithAttributes(attribute.String("host", host)))
}

// recordScrapeError counts one failed scrape.
func (m *Metrics) recordScrapeError(ctx context.Context, host string) {
	if m == nil {
		return
	}
	m.scrapeErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("host", host)))
}

// recordWriteError counts one failed write.
func (m *Metrics) recordWriteError(ctx context.Context, serialNumber string) {
	if m == nil {
		return
	}
	m.writeErrors.Add(ctx, 1, metric.WithAttributes(serialLabel(serialNumber)))
}

// recordReading sets the latest sensor gauges.
func (m *Metrics) recordReading(ctx context.Context, r *airgradient.MeasuresCurrentResponse) {
	if m == nil {
		return
	}
	attrs := metric.WithAttributes(serialLabel(r.Serialno))
	m.pm02.Record(ctx, r.Pm02, attrs)
	m.pm10.Record(ctx, r.Pm10, attrs)
	m.rco2.Record(ctx, r.Rco2, attrs)
}

// recordAQI sets the latest AQI gauge.
func (m *Metrics) recordAQI(ctx context.Context, a *timescale.AQI) {
	if m == nil {
		return
	}
	m.aqi.Record(ctx, a.AQI, metric.WithAttributes(serialLabel(a.SerialNumber)))
}

func serialLabel(serialNumber string) attribute.KeyValue {
	return attribute.String("serial_number", serialNumber)
}
