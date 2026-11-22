package formatter

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"
)

// Funcs are the functions a template may call: docker's (json, join, lower,
// upper, title, split, truncate, pad) and the CLI's own (ago, bytes, default).
func Funcs() template.FuncMap {
	return template.FuncMap{
		"json":     toJSON,
		"join":     join,
		"lower":    strings.ToLower,
		"upper":    strings.ToUpper,
		"title":    title,
		"split":    strings.Split,
		"truncate": truncate,
		"pad":      pad,
		"ago":      Ago,
		"bytes":    Bytes,
		"default":  orDefault,
	}
}

func toJSON(value any) (string, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func join(values any, separator string) string {
	switch v := values.(type) {
	case []string:
		return strings.Join(v, separator)
	case []any:
		parts := make([]string, len(v))
		for i, part := range v {
			parts[i] = fmt.Sprint(part)
		}

		return strings.Join(parts, separator)
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func title(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		first, size := utf8.DecodeRuneInString(word)
		words[i] = strings.ToUpper(string(first)) + word[size:]
	}

	return strings.Join(words, " ")
}

func truncate(value any, length int) string {
	s := fmt.Sprint(orDefault("", value))
	if utf8.RuneCountInString(s) <= length {
		return s
	}

	return string([]rune(s)[:length])
}

func pad(value any, before, after int) string {
	return strings.Repeat(" ", before) + fmt.Sprint(orDefault("", value)) + strings.Repeat(" ", after)
}

func orDefault(fallback any, value any) any {
	switch v := value.(type) {
	case nil:
		return fallback
	case string:
		if len(v) == 0 {
			return fallback
		}
	}

	return value
}

// Ago is how long ago a moment was, as docker says it: "3 hours ago". It
// takes a time or an RFC 3339 string, and says nothing of what is neither.
func Ago(value any) string {
	return agoFrom(time.Now(), value)
}

func agoFrom(now time.Time, value any) string {
	moment, ok := timeOf(value)
	if !ok || moment.IsZero() {
		return ""
	}

	elapsed := now.Sub(moment)
	if elapsed < 0 {
		return "in " + humanDuration(-elapsed)
	}

	if elapsed < time.Second {
		return "Less than a second ago"
	}

	return humanDuration(elapsed) + " ago"
}

func timeOf(value any) (time.Time, bool) {
	switch v := value.(type) {
	case time.Time:
		return v, true
	case *time.Time:
		if v == nil {
			return time.Time{}, false
		}

		return *v, true
	case string:
		moment, err := time.Parse(time.RFC3339Nano, v)

		return moment, err == nil
	}

	return time.Time{}, false
}

func humanDuration(d time.Duration) string {
	seconds := int(d.Seconds())
	minutes := int(d.Minutes())
	hours := int(math.Round(d.Hours()))

	switch {
	case seconds < 1:
		return "Less than a second"
	case seconds == 1:
		return "1 second"
	case seconds < 60:
		return fmt.Sprintf("%d seconds", seconds)
	case minutes == 1:
		return "About a minute"
	case minutes < 60:
		return fmt.Sprintf("%d minutes", minutes)
	case hours == 1:
		return "About an hour"
	case hours < 48:
		return fmt.Sprintf("%d hours", hours)
	case hours < 24*7*2:
		return fmt.Sprintf("%d days", hours/24)
	case hours < 24*30*2:
		return fmt.Sprintf("%d weeks", hours/24/7)
	case hours < 24*365*2:
		return fmt.Sprintf("%d months", hours/24/30)
	default:
		return fmt.Sprintf("%d years", hours/24/365)
	}
}

// Bytes is a size in bytes as binary units say it: "1.5GiB". It takes any
// number, or a string of one.
func Bytes(value any) string {
	size, ok := numberOf(value)
	if !ok {
		return ""
	}

	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}

	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}

	if unit == 0 {
		return strconv.FormatFloat(size, 'f', -1, 64) + units[unit]
	}

	return strconv.FormatFloat(math.Round(size*100)/100, 'f', -1, 64) + units[unit]
}

func numberOf(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case json.Number:
		f, err := v.Float64()

		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(v, 64)

		return f, err == nil
	}

	return 0, false
}
