//go:build windows

package benchmark_test

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	procQueryPerformanceCounter   *syscall.LazyProc
	procQueryPerformanceFrequency *syscall.LazyProc
	qpfFrequency                  int64
)

func init() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procQueryPerformanceCounter = kernel32.NewProc("QueryPerformanceCounter")
	procQueryPerformanceFrequency = kernel32.NewProc("QueryPerformanceFrequency")
	var freq int64
	r, _, _ := procQueryPerformanceFrequency.Call(uintptr(unsafe.Pointer(&freq)))
	if r != 0 && freq > 0 {
		qpfFrequency = freq
	}
}

type preciseTime int64

func nowPrecise() preciseTime {
	if qpfFrequency > 0 {
		var counter int64
		_, _, _ = procQueryPerformanceCounter.Call(uintptr(unsafe.Pointer(&counter)))
		return preciseTime(counter)
	}
	return preciseTime(time.Now().UnixNano())
}

func sincePrecise(t0 preciseTime) time.Duration {
	if qpfFrequency > 0 {
		var counter int64
		_, _, _ = procQueryPerformanceCounter.Call(uintptr(unsafe.Pointer(&counter)))
		ticks := counter - int64(t0)
		if ticks < 0 {
			ticks = 0
		}
		nanos := (ticks * 1_000_000_000) / qpfFrequency
		return time.Duration(nanos)
	}
	return time.Duration(time.Now().UnixNano() - int64(t0))
}
