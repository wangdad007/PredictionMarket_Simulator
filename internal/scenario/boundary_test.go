package scenario

import (
	"testing"
	"time"
)

func TestNextValidBeijingBoundaryMovesMondayToTuesdayMidnight(t *testing.T) {
	location, _ := time.LoadLocation(BeijingTimezone)
	input := time.Date(2026, time.July, 13, 15, 40, 0, 0, location)
	got := NextValidBeijingBoundary(input)
	if got.Weekday() != time.Tuesday || got.Hour() != 0 || got.Minute() != 0 || !got.After(input) {
		t.Fatalf("boundary=%s", got)
	}
}

func TestNormalizeMarketWindowUsesWholeDayValidBoundaries(t *testing.T) {
	location, _ := time.LoadLocation(BeijingTimezone)
	now := time.Date(2026, time.July, 17, 12, 0, 0, 0, location) // Friday
	start, end, err := NormalizeMarketWindow(now, 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if start.Weekday() != time.Tuesday || end.Weekday() != time.Thursday {
		t.Fatalf("window=%s -> %s", start, end)
	}
	if start.Hour() != 0 || end.Hour() != 0 || end.Sub(start) != 48*time.Hour {
		t.Fatalf("window=%s -> %s", start, end)
	}
}
