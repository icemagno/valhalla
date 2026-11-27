package logger

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

type blockingWriter struct {
	mu      sync.Mutex
	blockCh chan struct{}
	buf     bytes.Buffer
}

func (w *blockingWriter) Write(p []byte) (n int, err error) {
	if w.blockCh != nil {
		<-w.blockCh
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *blockingWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// TestLogger_ErrorAndCriticalNeverDroppedOnSaturation proves that when logQueue (10,000 items)
// is saturated, ERROR and CRITICAL messages are NEVER silently discarded.
func TestLogger_ErrorAndCriticalNeverDroppedOnSaturation(t *testing.T) {
	blockCh := make(chan struct{})
	blocker := &blockingWriter{blockCh: blockCh}

	blockLogger := &valhallaLogger{
		level:  LevelInfo,
		output: blocker,
		module: "BLOCKER",
	}

	// 1. Send 1 message to make the background worker enter blockingWriter.Write
	blockLogger.Info("blocking-first-message")
	// Give worker time to pick up this message and block on blockCh
	time.Sleep(50 * time.Millisecond)

	// 2. Saturate logQueue to its maximum capacity (cap(logQueue) = 10,000)
	dummyLogger := &valhallaLogger{
		level:  LevelDebug,
		output: blocker,
		module: "DUMMY",
	}
	for i := 0; i < cap(logQueue); i++ {
		dummyLogger.Debug("flood-%d", i)
	}

	if len(logQueue) < cap(logQueue) {
		t.Fatalf("Failed to saturate queue: len=%d, cap=%d", len(logQueue), cap(logQueue))
	}

	// 3. Now logQueue is 100% full. Issue an ERROR and a CRITICAL on a separate logger with its own buffer
	var errorBuf bytes.Buffer
	critLogger := &valhallaLogger{
		level:  LevelInfo,
		output: &errorBuf,
		module: "CRIT_MOD",
	}

	critLogger.Error("FATAL_ERROR_MSG_9999")
	critLogger.Critical("CRITICAL_SYSTEM_OUTAGE_8888")

	// Unblock background worker so it can drain
	close(blockCh)

	// On current flawed code, the ERROR and CRITICAL messages were discarded in the default: branch!
	out := errorBuf.String()
	if !strings.Contains(out, "FATAL_ERROR_MSG_9999") {
		t.Fatalf("SILENT DROP DETECTED: Error message was silently dropped when queue was saturated! Output: %q", out)
	}
	if !strings.Contains(out, "CRITICAL_SYSTEM_OUTAGE_8888") {
		t.Fatalf("SILENT DROP DETECTED: Critical message was silently dropped when queue was saturated! Output: %q", out)
	}

	// Wait for queue to drain cleanly
	Flush()
}

// TestLogger_FlushBlocksUntilDrainedWhenQueueFull proves that Flush() must not return
// immediately when logQueue is saturated; it must wait for all queued messages to be written.
func TestLogger_FlushBlocksUntilDrainedWhenQueueFull(t *testing.T) {
	blockCh := make(chan struct{})
	blocker := &blockingWriter{blockCh: blockCh}

	blockLogger := &valhallaLogger{
		level:  LevelInfo,
		output: blocker,
		module: "BLOCKER",
	}

	// 1. Block background worker
	blockLogger.Info("blocking-message")
	time.Sleep(50 * time.Millisecond)

	// 2. Fill logQueue completely
	for i := 0; i < cap(logQueue); i++ {
		blockLogger.Info("flood-%d", i)
	}

	if len(logQueue) < cap(logQueue) {
		t.Fatalf("Failed to saturate queue: len=%d, cap=%d", len(logQueue), cap(logQueue))
	}

	// 3. Call Flush in a separate goroutine while worker is STILL BLOCKED
	flushReturned := make(chan struct{})
	go func() {
		Flush()
		close(flushReturned)
	}()

	// If Flush returns while blockCh is NOT closed, Flush returned prematurely!
	select {
	case <-flushReturned:
		close(blockCh)
		t.Fatalf("FLUSH FAILED: Flush() returned immediately while queue was full without waiting for messages to drain!")
	case <-time.After(150 * time.Millisecond):
		// Expected behavior: Flush() is legitimately blocked waiting for queue to drain
	}

	// Now unblock worker
	close(blockCh)

	// Flush should now complete
	select {
	case <-flushReturned:
		// Succeeded in draining and unblocking
	case <-time.After(3 * time.Second):
		t.Fatalf("Flush() did not unblock after worker was released")
	}
}

// TestLogger_DroppedCountAndReset verifies that dropped non-critical logs are explicitly tracked
// when logQueue saturates, and can be inspected and reset.
func TestLogger_DroppedCountAndReset(t *testing.T) {
	ResetDroppedCount()

	blockCh := make(chan struct{})
	blocker := &blockingWriter{blockCh: blockCh}
	blockLogger := &valhallaLogger{
		level:  LevelInfo,
		output: blocker,
		module: "BLOCK_DROP",
	}

	blockLogger.Info("block")
	time.Sleep(50 * time.Millisecond)

	// Fill queue
	for i := 0; i < cap(logQueue); i++ {
		blockLogger.Info("fill-%d", i)
	}

	// Now try to send 10 non-critical messages (DEBUG, INFO, WARN)
	for i := 0; i < 10; i++ {
		blockLogger.Info("overflow-%d", i)
	}

	if dropped := DroppedCount(); dropped != 10 {
		close(blockCh)
		t.Fatalf("Expected 10 dropped messages, got %d", dropped)
	}

	ResetDroppedCount()
	if dropped := DroppedCount(); dropped != 0 {
		close(blockCh)
		t.Fatalf("Expected 0 dropped messages after reset, got %d", dropped)
	}

	close(blockCh)
	Flush()
}

// TestLogger_FlushTimeout verifies that FlushTimeout returns false if the queue cannot be flushed
// within the allotted deadline, and true when it can.
func TestLogger_FlushTimeout(t *testing.T) {
	blockCh := make(chan struct{})
	blocker := &blockingWriter{blockCh: blockCh}
	blockLogger := &valhallaLogger{
		level:  LevelInfo,
		output: blocker,
		module: "BLOCK_TIMEOUT",
	}

	blockLogger.Info("block")
	time.Sleep(50 * time.Millisecond)

	for i := 0; i < cap(logQueue); i++ {
		blockLogger.Info("fill-%d", i)
	}

	// Attempt flush with 50ms timeout while worker is blocked -> MUST return false
	ok := FlushTimeout(50 * time.Millisecond)
	if ok {
		close(blockCh)
		t.Fatalf("Expected FlushTimeout to return false while worker blocked, got true")
	}

	// Unblock worker
	close(blockCh)

	// Attempt flush with generous timeout -> MUST return true
	ok = FlushTimeout(3 * time.Second)
	if !ok {
		t.Fatalf("Expected FlushTimeout to return true after unblocking, got false")
	}
}
