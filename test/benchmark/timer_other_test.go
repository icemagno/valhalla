//go:build !windows

package benchmark_test

import (
	"time"
)

type preciseTime time.Time

func nowPrecise() preciseTime {
	return preciseTime(time.Now())
}

func sincePrecise(t0 preciseTime) time.Duration {
	return time.Since(time.Time(t0))
}
