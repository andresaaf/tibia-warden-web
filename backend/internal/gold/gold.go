// Package gold parses and formats Tibia gold amounts written the way players
// type them: "30k", "1.5kk", "30000", "30,000".
package gold

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// Max caps a parsed amount (100kk) to reject typos and overflow.
const Max int64 = 100_000_000

// ErrInvalid is returned for input that isn't a gold amount.
var ErrInvalid = errors.New("invalid gold amount")

// Parse reads a gold amount. It accepts plain numbers (commas, dots-as-decimals
// and spaces allowed) with an optional case-insensitive "k" (×1000) or "kk"/"m"
// (×1,000,000) suffix. The result is rounded to whole gold and must be within
// 0..Max.
func Parse(s string) (int64, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSuffix(s, "gp")
	mult := 1.0
	switch {
	case strings.HasSuffix(s, "kk"):
		mult, s = 1_000_000, strings.TrimSuffix(s, "kk")
	case strings.HasSuffix(s, "m"):
		mult, s = 1_000_000, strings.TrimSuffix(s, "m")
	case strings.HasSuffix(s, "k"):
		mult, s = 1000, strings.TrimSuffix(s, "k")
	}
	if s == "" || strings.ContainsAny(s, "+-eE") {
		return 0, ErrInvalid
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, ErrInvalid
	}
	v := math.Round(f * mult)
	if v < 0 || v > float64(Max) {
		return 0, ErrInvalid
	}
	return int64(v), nil
}

// Format renders an amount Tibia-style, mirroring the frontend's formatK:
// 999 → "999", 1560 → "1.6k", 30000 → "30k", 2500000 → "2.5kk".
func Format(n int64) string {
	abs := n
	if abs < 0 {
		abs = -abs
	}
	if abs < 1000 {
		return strconv.FormatInt(n, 10)
	}
	div, suffix := 1000.0, "k"
	if abs >= 1_000_000 {
		div, suffix = 1_000_000, "kk"
	}
	scaled := float64(n) / div
	var rounded float64
	if math.Abs(scaled) < 10 {
		rounded = math.Round(scaled*10) / 10
	} else {
		rounded = math.Round(scaled)
	}
	return strconv.FormatFloat(rounded, 'f', -1, 64) + suffix
}
