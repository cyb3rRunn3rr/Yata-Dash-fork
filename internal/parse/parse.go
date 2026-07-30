// Package parse provides shared parsing utilities for sizes, seed times, and numbers.
// These are used by the stats fetching layer and the history recorder.
package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var sizeRe = regexp.MustCompile(`(?i)([\d.]+)\s*(B|[KMGTP]i?B)`)

var sizeFactors = map[string]float64{
	"b":   1.0 / (1024 * 1024 * 1024),
	"kib": 1.0 / (1024 * 1024),
	"mib": 1.0 / 1024,
	"gib": 1.0,
	"tib": 1024.0,
	"pib": 1024.0 * 1024.0,
	// Decimal-labelled units (TBDev-family sites render "1.005 TB"): tracker
	// software near-universally computes 1024-based sizes but mislabels them,
	// so these map to the same factors as their -iB counterparts.
	"kb": 1.0 / (1024 * 1024),
	"mb": 1.0 / 1024,
	"gb": 1.0,
	"tb": 1024.0,
	"pb": 1024.0 * 1024.0,
}

// SizeToGiB parses a human-readable size string (e.g. "3.14 TiB") into GiB.
// Returns nil when the input is empty or unparseable.
func SizeToGiB(s string) *float64 {
	m := sizeRe.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return nil
	}
	f, ok := sizeFactors[strings.ToLower(m[2])]
	if !ok {
		return nil
	}
	result := v * f
	return &result
}

// SeedTimeToSeconds parses a seed time string (e.g. "3M 6D 22h 23m 13s") to seconds.
// Returns nil for empty/invalid input.
// Also accepts a plain integer/float as raw seconds.
func SeedTimeToSeconds(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Plain numeric — treat as seconds directly
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		if v > 0 {
			return &v
		}
		return nil
	}
	tokens := durTokenRe.FindAllStringSubmatch(s, -1)
	if len(tokens) == 0 {
		return nil
	}
	units := make([]durUnit, len(tokens))
	for i, t := range tokens {
		units[i] = classifyDurUnit(t[2])
	}
	// A bare lowercase "m" means minutes in English layouts and months in
	// Portuguese ones ("2m 4d 13h 59m 13s" is 2 months … 59 minutes). UNIT3D
	// renders units in descending order, so an "m" with a day, week or hour
	// after it occupies the month slot; every later one is minutes.
	for i, t := range tokens {
		if units[i] != durUnknown || t[2] != "m" {
			continue
		}
		units[i] = durMinute
		for _, later := range units[i+1:] {
			if later == durDay || later == durWeek || later == durHour {
				units[i] = durMonth
				break
			}
		}
	}
	var total float64
	for i, t := range tokens {
		v, err := strconv.ParseFloat(t[1], 64)
		if err != nil {
			continue
		}
		total += v * durSeconds[units[i]]
	}
	if total <= 0 {
		return nil
	}
	return &total
}

// durTokenRe splits a duration into value/unit pairs, e.g. "178a 2m 4d 20h
// 52m 38s" → (178,a) (2,m) (4,d) (20,h) (52,m) (38,s). Every occurrence of a
// unit counts, so a string carrying both a month and a minute "m" adds both.
var durTokenRe = regexp.MustCompile(`([\d.]+)\s*([A-Za-zÀ-ÿ]+)`)

type durUnit int

const (
	durUnknown durUnit = iota
	durYear
	durMonth
	durWeek
	durDay
	durHour
	durMinute
	durSecond
)

var durSeconds = map[durUnit]float64{
	durUnknown: 0,
	durYear:    365.25 * 86400,
	durMonth:   30.44 * 86400,
	durWeek:    7 * 86400,
	durDay:     86400,
	durHour:    3600,
	durMinute:  60,
	durSecond:  1,
}

// classifyDurUnit maps a unit token to its duration unit. Case matters where
// tracker layouts rely on it: "M" is months against "m" minutes in English,
// and "S" is semanas (weeks) against "s" seconds on Portuguese sites. A bare
// lowercase "m" is ambiguous and gets resolved by position in the caller.
func classifyDurUnit(raw string) durUnit {
	switch raw {
	case "y", "Y", "a", "A", "ano", "anos", "year", "years":
		return durYear
	case "M", "mo", "mes", "meses", "month", "months":
		return durMonth
	case "w", "W", "S", "sem", "semana", "semanas", "week", "weeks":
		return durWeek
	case "d", "D", "dia", "dias", "day", "days":
		return durDay
	case "h", "H", "hora", "horas", "hour", "hours":
		return durHour
	case "min", "mins", "minuto", "minutos", "minute", "minutes":
		return durMinute
	case "s", "seg", "segs", "segundo", "segundos", "sec", "secs", "second", "seconds":
		return durSecond
	}
	return durUnknown
}

// AnyFloat converts any numeric JSON value to float64.
func AnyFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case string:
		f, _ := strconv.ParseFloat(strings.ReplaceAll(n, ",", ""), 64)
		return f
	}
	return 0
}

// BytesToSize converts a raw byte count (as returned by the Gazelle API) to a
// human-readable size string with exactly 2 decimal places, e.g. "25.52 GiB".
func BytesToSize(b int64) string {
	if b <= 0 {
		return "0.00 B"
	}
	const (
		kib = int64(1024)
		mib = kib * 1024
		gib = mib * 1024
		tib = gib * 1024
		pib = tib * 1024
	)
	switch {
	case b >= pib:
		return fmt.Sprintf("%.2f PiB", float64(b)/float64(pib))
	case b >= tib:
		return fmt.Sprintf("%.2f TiB", float64(b)/float64(tib))
	case b >= gib:
		return fmt.Sprintf("%.2f GiB", float64(b)/float64(gib))
	case b >= mib:
		return fmt.Sprintf("%.2f MiB", float64(b)/float64(mib))
	case b >= kib:
		return fmt.Sprintf("%.2f KiB", float64(b)/float64(kib))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// NormalizeSeedSize re-formats a size string to exactly 2 decimal places.
// Handles scraped values like "1.239 TiB" → "1.24 TiB".
// Leaves strings that don't match a known unit pattern unchanged.
func NormalizeSeedSize(s string) string {
	m := sizeRe.FindStringSubmatch(s)
	if m == nil {
		return s
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return s
	}
	return fmt.Sprintf("%.2f %s", v, m[2])
}

// FormatSeedTime formats seconds into a human string like "1Y 2M 3D 4h 5m 6s".
func FormatSeedTime(totalSec float64) string {
	t := int64(totalSec)
	if t <= 0 {
		return "0s"
	}
	steps := []struct {
		sec int64
		u   string
	}{
		{31536000, "Y"}, {2592000, "M"}, {604800, "W"},
		{86400, "D"}, {3600, "h"}, {60, "m"}, {1, "s"},
	}
	var parts []string
	for _, s := range steps {
		v := t / s.sec
		t -= v * s.sec
		if v > 0 {
			parts = append(parts, fmt.Sprintf("%d%s", v, s.u))
		}
	}
	if len(parts) == 0 {
		return "0s"
	}
	return strings.Join(parts, " ")
}
