package timescale

import (
	"math"
	"testing"
)

// TestCelsiusToFahrenheit checks the conversion the insert applies to the
// firmware's Celsius temperatures.
func TestCelsiusToFahrenheit(t *testing.T) {
	cases := []struct{ c, f float64 }{
		{0, 32},
		{100, 212},
		{-40, -40},
		{21.5, 70.7},
	}
	for _, tc := range cases {
		if got := celsiusToFahrenheit(tc.c); math.Abs(got-tc.f) > 1e-9 {
			t.Errorf("celsiusToFahrenheit(%v) = %v, want %v", tc.c, got, tc.f)
		}
	}
}

// TestAQIFromAverages checks the pollutant selection and category mapping.
func TestAQIFromAverages(t *testing.T) {
	// 30.2 µg/m3 PM2.5 is AQI 89 (Moderate); 20 µg/m3 PM10 is AQI 19 (Good).
	a, err := aqiFromAverages("abc", 30.2, 20)
	if err != nil {
		t.Fatal(err)
	}
	if a.SerialNumber != "abc" {
		t.Errorf("serial: got %q", a.SerialNumber)
	}
	if a.PrimaryPollutant != "PM2.5" {
		t.Errorf("primary: got %q, want PM2.5", a.PrimaryPollutant)
	}
	if a.AQI != 89 || a.Designation != "Moderate" {
		t.Errorf("aqi: got %d %q, want 89 Moderate", a.AQI, a.Designation)
	}

	// 5 µg/m3 PM2.5 is AQI 21; 200 µg/m3 PM10 is AQI 123 (Unhealthy for
	// Sensitive Groups), so PM10 wins.
	b, err := aqiFromAverages("abc", 5, 200)
	if err != nil {
		t.Fatal(err)
	}
	if b.PrimaryPollutant != "PM10.0" {
		t.Errorf("primary: got %q, want PM10.0", b.PrimaryPollutant)
	}
	if b.AQI <= a.AQI {
		t.Errorf("expected a higher AQI from heavy PM10, got %d", b.AQI)
	}

	// Clean air with equal sub-indices ties to PM10.
	c, err := aqiFromAverages("abc", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if c.PrimaryPollutant != "PM10.0" || c.AQI != 0 || c.Designation != "Good" {
		t.Errorf("tie: got %q %d %q", c.PrimaryPollutant, c.AQI, c.Designation)
	}

	// A negative average is off the scale and must be an error.
	if _, err := aqiFromAverages("abc", -1, 0); err == nil {
		t.Errorf("expected an error for a negative concentration")
	}
}
