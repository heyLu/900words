package main

import (
	"fmt"
	"time"
)

func main() {
	now := time.Now()
	for _, d := range daysOfMonth(now) {
		sep := ""
		if now.Day() == d.Day() {
			sep = "*"
		}
		fmt.Printf("%s%d%s ", sep, d.Day(), sep)
	}
	fmt.Println()
}

func daysOfMonth(t time.Time) []time.Time {
	s := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	e := s.AddDate(0, 1, 0)
	ts := make([]time.Time, 0, 31)
	for s.Before(e) {
		ts = append(ts, s)
		s = s.AddDate(0, 0, 1)
	}
	return ts
}
