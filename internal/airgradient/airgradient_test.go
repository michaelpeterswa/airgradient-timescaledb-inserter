package airgradient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sampleBody = `{
	"wifi": -55, "serialno": "abc123", "rco2": 612, "pm01": 1.2, "pm02": 2.5,
	"pm10": 3.1, "pm003Count": 410, "atmp": 21.5, "rhum": 48, "atmpCompensated": 20.9,
	"rhumCompensated": 50, "tvocIndex": 100, "tvocRaw": 30500, "noxIndex": 1,
	"noxRaw": 16000, "boot": 3, "bootCount": 3, "firmware": "3.1.9", "model": "I-9PSL"
}`

// TestGetCurrentMeasures checks that the client hits the right path and decodes
// the firmware's JSON into the response struct.
func TestGetCurrentMeasures(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleBody))
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	m, err := NewClient(srv.Client()).GetCurrentMeasures(context.Background(), host)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/measures/current" {
		t.Errorf("path: got %q, want /measures/current", gotPath)
	}
	if m.Serialno != "abc123" {
		t.Errorf("serialno: got %q", m.Serialno)
	}
	if m.Pm02 != 2.5 || m.Rco2 != 612 || m.Atmp != 21.5 {
		t.Errorf("values: got pm02=%v rco2=%v atmp=%v", m.Pm02, m.Rco2, m.Atmp)
	}
	if m.Firmware != "3.1.9" || m.Model != "I-9PSL" {
		t.Errorf("strings: got firmware=%q model=%q", m.Firmware, m.Model)
	}
}

// TestGetCurrentMeasuresErrors checks that a non-200 status and a malformed
// body both surface as errors instead of a zero-valued reading.
func TestGetCurrentMeasuresErrors(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"server error": func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		},
		"bad json": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"serialno": `))
		},
	}
	for name, h := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(h)
			defer srv.Close()
			host := strings.TrimPrefix(srv.URL, "http://")
			if _, err := NewClient(srv.Client()).GetCurrentMeasures(context.Background(), host); err == nil {
				t.Errorf("expected an error")
			}
		})
	}
}

// TestGetCurrentMeasuresContext checks that a cancelled context aborts the call.
func TestGetCurrentMeasuresContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(sampleBody))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	host := strings.TrimPrefix(srv.URL, "http://")
	if _, err := NewClient(srv.Client()).GetCurrentMeasures(ctx, host); err == nil {
		t.Errorf("expected an error from a cancelled context")
	}
}
