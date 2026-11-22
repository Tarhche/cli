package workload

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

// layouts are the times a moment is given as, after a duration: RFC 3339,
// and the local time of day or a date without a zone.
var layouts = []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02"}

// momentOf is the moment value says, as docker's --since and --until take
// one: a duration before now (42m, 2h), a time (2026-11-22T10:00:00Z, or
// local without a zone), or a Unix time in seconds. Nothing is no moment.
func momentOf(value string, now time.Time) (time.Time, error) {
	if len(value) == 0 {
		return time.Time{}, nil
	}

	if ago, err := time.ParseDuration(value); err == nil {
		return now.Add(-ago), nil
	}

	for _, layout := range layouts {
		if moment, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return moment, nil
		}
	}

	if seconds, err := strconv.ParseFloat(value, 64); err == nil && !math.IsInf(seconds, 0) && !math.IsNaN(seconds) {
		whole, fraction := math.Modf(seconds)

		return time.Unix(int64(whole), int64(fraction*float64(time.Second))), nil
	}

	return time.Time{}, fmt.Errorf("%q is not a time: give a duration ago (42m), a time (2026-11-22T10:00:00Z) or a Unix time", value)
}
