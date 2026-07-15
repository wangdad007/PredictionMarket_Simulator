package scenario

import (
	"fmt"
	"math"
	"time"
)

const BeijingTimezone = "Asia/Shanghai"

func NextValidBeijingBoundary(input time.Time) time.Time {
	location, _ := time.LoadLocation(BeijingTimezone)
	local := input.In(location)
	candidate := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	if !candidate.After(local) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	for !isSettlementWeekday(candidate.Weekday()) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
}

func NormalizeMarketWindow(now time.Time, requested time.Duration) (time.Time, time.Time, error) {
	if requested < 24*time.Hour || requested > 4*24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("market duration must be between 1 and 4 whole days")
	}
	days := int(math.Ceil(float64(requested) / float64(24*time.Hour)))
	start := NextValidBeijingBoundary(now)
	for {
		end := start.AddDate(0, 0, days)
		if isSettlementWeekday(end.Weekday()) {
			return start, end, nil
		}
		start = NextValidBeijingBoundary(start)
	}
}

func NormalizeStreakWindow(now time.Time, requested time.Duration) (time.Time, time.Time, error) {
	if requested < 24*time.Hour || requested > 4*24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("streak duration must be between 1 and 4 whole days")
	}
	days := int(math.Ceil(float64(requested) / float64(24*time.Hour)))
	start := NextValidBeijingBoundary(now)
	for int(start.Weekday())+days > int(time.Saturday) {
		start = start.AddDate(0, 0, 1)
		for start.Weekday() != time.Tuesday {
			start = start.AddDate(0, 0, 1)
		}
	}
	return start, start.AddDate(0, 0, days), nil
}

func IsValidBeijingBoundary(value time.Time) bool {
	location, _ := time.LoadLocation(BeijingTimezone)
	local := value.In(location)
	return local.Hour() == 0 && local.Minute() == 0 && local.Second() == 0 &&
		local.Nanosecond() == 0 && isSettlementWeekday(local.Weekday())
}

func isSettlementWeekday(day time.Weekday) bool {
	return day >= time.Tuesday && day <= time.Saturday
}
