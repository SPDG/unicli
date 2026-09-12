package network

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const maxTrafficRangeSlack = 2 * time.Hour

// LoadLocationIANA loads an IANA timezone. Empty becomes UTC.
func LoadLocationIANA(name string) (*time.Location, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return time.UTC, "UTC", nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, "", fmt.Errorf("invalid timezone %q", name)
	}
	return loc, name, nil
}

// ParseTrafficTime parses RFC3339, a local date, local datetime, or unix seconds/millis.
// Date-only values are midnight in loc. endOfDate makes a date-only value the next local midnight (exclusive end).
func ParseTrafficTime(raw string, loc *time.Location, endOfDate bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if loc == nil {
		loc = time.UTC
	}
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.In(loc), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.In(loc), nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", raw, loc); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", raw, loc); err == nil {
		if endOfDate {
			return t.AddDate(0, 0, 1), nil
		}
		return t, nil
	}
	if isAllDigits(raw) {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid unix timestamp %q", raw)
		}
		switch {
		case n > 1_000_000_000_000:
			return time.UnixMilli(n).In(loc), nil
		case n > 1_000_000_000:
			return time.Unix(n, 0).In(loc), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time %q (use RFC3339 or YYYY-MM-DD)", raw)
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// MonthToDateWindow is [first of month 00:00, observedAt) in loc.
func MonthToDateWindow(observedAt time.Time, loc *time.Location) (start, end time.Time) {
	if loc == nil {
		loc = time.UTC
	}
	now := observedAt.In(loc)
	start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	return start, now
}

func ValidateTrafficRange(start, end time.Time, loc *time.Location) error {
	if loc == nil {
		loc = time.UTC
	}
	start = start.In(loc)
	end = end.In(loc)
	if !start.Before(end) {
		return fmt.Errorf("start must be before end")
	}
	if end.Sub(start) > time.Duration(MaxTrafficCalendarDays)*24*time.Hour+maxTrafficRangeSlack {
		return fmt.Errorf("range exceeds %d days", MaxTrafficCalendarDays)
	}
	days := 0
	for t := start; t.Before(end) && days <= MaxTrafficCalendarDays+1; t = t.AddDate(0, 0, 1) {
		days++
	}
	if days > MaxTrafficCalendarDays {
		return fmt.Errorf("range exceeds %d calendar days", MaxTrafficCalendarDays)
	}
	return nil
}

func formatTrafficTime(t time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	if t.IsZero() {
		return ""
	}
	return t.In(loc).Format(time.RFC3339)
}

func intervalJSON(start, end time.Time, loc *time.Location) TrafficInterval {
	return TrafficInterval{
		Start: formatTrafficTime(start, loc),
		End:   formatTrafficTime(end, loc),
	}
}

type localDay struct {
	Start time.Time
	End   time.Time
}

func localCalendarDay(t time.Time, loc *time.Location) (localDay, bool) {
	if loc == nil {
		loc = time.UTC
	}
	x := t.In(loc)
	if x.Hour() != 0 || x.Minute() != 0 || x.Second() != 0 || x.Nanosecond() != 0 {
		return localDay{}, false
	}
	start := time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)
	return localDay{Start: start, End: end}, true
}

func resolutionDuration(res string) time.Duration {
	switch res {
	case trafficRes5Min:
		return 5 * time.Minute
	case trafficResHour:
		return time.Hour
	case trafficResDay:
		return 24 * time.Hour
	default:
		return 0
	}
}

func dstCalendarDay(day localDay) bool {
	return day.End.Sub(day.Start) != 24*time.Hour
}

// resolutionQueryWindow narrows fine-grained report queries. Month-long hourly
// requests can omit today's buckets on some controllers; daily still covers
// complete days.
func resolutionQueryWindow(res string, q TrafficQuery) (startMs, endMs int64) {
	end := q.End
	if !q.ObservedAt.IsZero() && q.ObservedAt.Before(end) {
		end = q.ObservedAt
	}
	start := q.Start
	switch res {
	case trafficResHour:
		lookback := end.Add(-8 * 24 * time.Hour)
		if lookback.After(start) {
			start = lookback
		}
	case trafficRes5Min:
		lookback := end.Add(-24 * time.Hour)
		if lookback.After(start) {
			start = lookback
		}
	}
	return start.UnixMilli(), end.UnixMilli()
}
