package internal

import (
	"fmt"
	"time"
)

func relativeDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	if h > 0 {
		return fmt.Sprintf("-%dm", h*60+m) // convert to minutes
	}
	if m == 0 {
		return "-1m" // always return at least 1m ago
	}
	return fmt.Sprintf("-%dm", m)
}
