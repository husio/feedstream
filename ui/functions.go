package ui

import (
	"fmt"
	"time"
)

func fnTimesince(t time.Time) string {
	now := time.Now()

	diff := now.Sub(t)
	if diff < 0 {
		panic("not implemented")
	}

	switch {
	case diff < 2*time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%d minutes ago", diff/time.Minute)
	case diff < 24*time.Hour:
		h := diff / time.Hour
		if h == 1 {
			return "an hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case diff > 364*24*time.Hour:
		y := diff / (364 * 24 * time.Hour)
		if y == 1 {
			return "year ago"
		}
		return fmt.Sprintf("%d years ago", y)
	default:
		d := diff / (24 * time.Hour)
		if d == 1 {
			return "day ago"
		}
		return fmt.Sprintf("%d days ago", d)
	}
}
