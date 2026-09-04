package inserter

import (
	"context"
	"testing"
	"time"

	"github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/config"
)

// TestWithTimeoutDetachesFromCancel checks that a write started under a
// cancelled parent still gets its own timeout, so shutdown does not abandon
// a write mid-flight.
func TestWithTimeoutDetachesFromCancel(t *testing.T) {
	r := &Runner{cfg: &config.Config{InsertTimeout: time.Second}}

	parent, cancel := context.WithCancel(context.Background())
	cancel()

	err := r.withTimeout(parent, func(c context.Context) error {
		if c.Err() != nil {
			t.Errorf("child context already done: %v", c.Err())
		}
		if _, ok := c.Deadline(); !ok {
			t.Errorf("child context has no deadline")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestNilMetricsAreSafe checks that a Runner built without metrics does not
// panic when it records.
func TestNilMetricsAreSafe(t *testing.T) {
	var m *Metrics
	ctx := context.Background()
	m.recordScrape(ctx, "h")
	m.recordScrapeError(ctx, "h")
	m.recordWriteError(ctx, "s")
	m.recordReading(ctx, nil)
	m.recordAQI(ctx, nil)
}
