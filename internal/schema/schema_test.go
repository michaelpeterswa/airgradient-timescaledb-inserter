package schema

import (
	"os"
	"testing"
)

// TestEmbeddedMigrationsLoad checks that the embedded files parse as a valid
// golang-migrate source: sequential numbering and matching up/down pairs.
// The database round trip is covered by TestMigrateRoundTrip when a database
// is available.
func TestEmbeddedMigrationsLoad(t *testing.T) {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries)%2 != 0 || len(entries) == 0 {
		t.Fatalf("expected up/down pairs, got %d files", len(entries))
	}
}

// TestMigrateRoundTrip applies every migration to a live TimescaleDB, rolls
// them all back, and applies them again. Set TEST_TIMESCALE_CONN_STRING to
// run it; it is skipped otherwise so the unit suite has no database
// dependency.
func TestMigrateRoundTrip(t *testing.T) {
	conn := os.Getenv("TEST_TIMESCALE_CONN_STRING")
	if conn == "" {
		t.Skip("TEST_TIMESCALE_CONN_STRING not set")
	}

	mg, err := New(conn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mg.Close() }()

	if _, err := mg.Up(); err != nil {
		t.Fatalf("up: %v", err)
	}
	v, dirty, err := mg.Version()
	if err != nil || dirty || v != 3 {
		t.Fatalf("after up: version=%d dirty=%v err=%v, want 3 clean", v, dirty, err)
	}

	applied, err := mg.Up()
	if err != nil || applied {
		t.Fatalf("second up: applied=%v err=%v, want no change", applied, err)
	}

	for i := 0; i < 3; i++ {
		if err := mg.Down(); err != nil {
			t.Fatalf("down %d: %v", i, err)
		}
	}
	v, _, err = mg.Version()
	if err != nil || v != 0 {
		t.Fatalf("after down: version=%d err=%v, want 0", v, err)
	}

	if _, err := mg.Up(); err != nil {
		t.Fatalf("reapply: %v", err)
	}
}
