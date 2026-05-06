package timestamps

import (
	"testing"
	"time"
)

func TestClockFuncReturnsUTC(t *testing.T) {
	local := time.FixedZone("local", 2*60*60)
	clock := ClockFunc(func() time.Time {
		return time.Date(2026, 5, 6, 14, 0, 0, 0, local)
	})

	got := clock.Now()
	if got.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %v", got.Location())
	}
	if got.Hour() != 12 {
		t.Fatalf("expected UTC hour 12, got %d", got.Hour())
	}
}

func TestRFC3339FormatsUTC(t *testing.T) {
	local := time.FixedZone("local", 2*60*60)
	value := time.Date(2026, 5, 6, 14, 0, 0, 123, local)

	if got := RFC3339(value); got != "2026-05-06T12:00:00.000000123Z" {
		t.Fatalf("unexpected timestamp %q", got)
	}
}
