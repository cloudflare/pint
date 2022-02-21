package output

import (
	"fmt"
	"math"
	"strings"
	"time"
)

func HumanizeDuration(d time.Duration) string {
	weeks := int64(d.Hours() / (7 * 24))
	days := int64(math.Mod(d.Hours(), 7*24) / 24)
	hours := int64(math.Mod(d.Hours(), 24))
	minutes := int64(math.Mod(d.Minutes(), 60))
	seconds := int64(math.Mod(d.Seconds(), 60))
	ms := int64(math.Mod(float64(d.Milliseconds()), 1000))

	chunks := []struct {
		singularName string
		amount       int64
	}{
		{"w", weeks},
		{"d", days},
		{"h", hours},
		{"m", minutes},
		{"s", seconds},
		{"ms", ms},
	}

	parts := []string{}

	for _, chunk := range chunks {
		if chunk.amount > 0 {
			parts = append(parts, fmt.Sprintf("%d%s", chunk.amount, chunk.singularName))
		}
	}

	if len(parts) == 0 {
		return "0"
	}

	return strings.Join(parts, "")
}

func HumanizeBytes(b int) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}
