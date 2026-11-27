package logger

import (
	"bytes"
	"sync"
	"testing"
)

func TestLogger_SharedWriterConcurrentLoggers_RaceFree(t *testing.T) {
	var sharedBuf bytes.Buffer
	l1 := NewLogger("MOD1")
	l2 := NewLogger("MOD2")
	l1.SetOutput(&sharedBuf)
	l2.SetOutput(&sharedBuf)
	l1.SetLevel(LevelInfo)
	l2.SetLevel(LevelInfo)

	// Exercise both async worker path and synchronous writeEntry path concurrently
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			l1.Info("msg1 %d", i)
			writeEntry(&sharedBuf, logEntry{timestamp: "00:00:00.000", level: "ERROR", module: "MOD1", message: "sync1"})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			l2.Info("msg2 %d", i)
			writeEntry(&sharedBuf, logEntry{timestamp: "00:00:00.000", level: "ERROR", module: "MOD2", message: "sync2"})
		}
	}()
	wg.Wait()
	Flush()
}
