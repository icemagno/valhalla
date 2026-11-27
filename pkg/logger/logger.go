/*
 * Projeto Valhalla - High-Performance Sovereign HLA-4 (IEEE 1516-2025) Run-Time Infrastructure
 * Lead Developer: Carlos Magno O. Abreu (magno.mabreu@gmail.com)
 */

package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// GlobalConfigAccessor defines the interface required by the logger to obtain the global log level.
type GlobalConfigAccessor interface {
	GetLogLevel() int
}

var (
	configAccessorMu sync.RWMutex
	configAccessor   GlobalConfigAccessor
)

// SetConfigAccessor registers a global configuration provider.
func SetConfigAccessor(a GlobalConfigAccessor) {
	configAccessorMu.Lock()
	defer configAccessorMu.Unlock()
	configAccessor = a
}

func getConfigLogLevel() (LogLevel, bool) {
	configAccessorMu.RLock()
	defer configAccessorMu.RUnlock()
	if configAccessor != nil {
		return LogLevel(configAccessor.GetLogLevel()), true
	}
	return LevelInfo, false
}

// LogLevel defines message severity.
type LogLevel int

const (
	LevelTrace LogLevel = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelCritical
	LevelNone
)

func (l LogLevel) String() string {
	switch l {
	case LevelTrace:
		return "TRACE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO "
	case LevelWarn:
		return "WARN "
	case LevelError:
		return "ERROR"
	case LevelCritical:
		return "CRIT "
	default:
		return "UNKNOWN"
	}
}

// ParseLogLevel converts a string to LogLevel.
func ParseLogLevel(level string) LogLevel {
	switch level {
	case "trace":
		return LevelTrace
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	case "critical":
		return LevelCritical
	default:
		return LevelInfo
	}
}

// Logger defines Valhalla's asynchronous observability interface.
type Logger interface {
	Critical(format string, args ...interface{})
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
	Trace(format string, args ...interface{})
	SetLevel(level LogLevel)
	GetLevel() LogLevel
	SetOutput(w io.Writer)
}

// valhallaLogger is the high-performance decoupled asynchronous Logger implementation.
type valhallaLogger struct {
	mu     sync.RWMutex
	level  LogLevel
	output io.Writer
	outMu  sync.Mutex // Guards writes to this specific logger's output
	module string
}

type logEntry struct {
	timestamp string
	level     string
	module    string
	message   string
	output    io.Writer
	logger    *valhallaLogger
	flushDone chan struct{}
}

var (
	globalLogger    Logger
	once            sync.Once
	logQueue        = make(chan logEntry, 10000) // Decoupled FIFO buffer
	droppedLogCount uint64                      // Cumulative count of dropped non-critical logs on saturation
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
)

func formatEntry(entry logEntry) string {
	color := ""
	switch entry.level {
	case "WARN ":
		color = colorYellow
	case "ERROR", "CRIT ":
		color = colorRed
	case "DEBUG":
		color = colorGray
	case "TRACE":
		color = colorBlue
	}

	if color != "" {
		return fmt.Sprintf("[%s] %s[%s]%s [%s] %s\n",
			entry.timestamp, color, entry.level, colorReset, entry.module, entry.message)
	}
	return fmt.Sprintf("[%s] [%s] [%s] %s\n",
		entry.timestamp, entry.level, entry.module, entry.message)
}

func writeEntry(out io.Writer, entry logEntry) {
	if out == nil {
		out = os.Stdout
	}
	line := formatEntry(entry)
	_, _ = io.WriteString(out, line)
}

func init() {
	// Background asynchronous worker for out-of-hotpath logging I/O
	go func() {
		for entry := range logQueue {
			if entry.flushDone != nil {
				close(entry.flushDone)
				continue
			}

			if entry.logger != nil {
				entry.logger.outMu.Lock()
				writeEntry(entry.output, entry)
				entry.logger.outMu.Unlock()
			} else {
				writeEntry(entry.output, entry)
			}
		}
	}()
}

// Flush blocks until all currently queued messages are written to output.
func Flush() {
	done := make(chan struct{})
	logQueue <- logEntry{flushDone: done}
	<-done
}

// FlushTimeout blocks until all currently queued messages are written or timeout expires.
func FlushTimeout(timeout time.Duration) bool {
	done := make(chan struct{})
	select {
	case logQueue <- logEntry{flushDone: done}:
		select {
		case <-done:
			return true
		case <-time.After(timeout):
			return false
		}
	case <-time.After(timeout):
		return false
	}
}

// DroppedCount returns the number of non-critical messages dropped due to queue saturation.
func DroppedCount() uint64 {
	return atomic.LoadUint64(&droppedLogCount)
}

// ResetDroppedCount resets the counter of dropped log messages.
func ResetDroppedCount() {
	atomic.StoreUint64(&droppedLogCount, 0)
}

// InitGlobalLogger initializes the global logger without a module name.
func InitGlobalLogger(level LogLevel) Logger {
	once.Do(func() {
		globalLogger = &valhallaLogger{
			level:  level,
			output: os.Stdout,
			module: "VALHALLA",
		}
	})
	return globalLogger
}

// NewLogger creates a logger instance for a specific module.
func NewLogger(module string) Logger {
	level := LevelInfo
	if l, ok := getConfigLogLevel(); ok {
		level = l
	}
	return &valhallaLogger{
		level:  level,
		output: os.Stdout,
		module: module,
	}
}

// GetGlobal returns the global logger instance.
func GetGlobal() Logger {
	if globalLogger == nil {
		level := LevelInfo
		if l, ok := getConfigLogLevel(); ok {
			level = l
		}
		InitGlobalLogger(level)
	}
	return globalLogger
}

func (l *valhallaLogger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *valhallaLogger) GetLevel() LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

func (l *valhallaLogger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

func (l *valhallaLogger) log(level LogLevel, format string, args ...interface{}) {
	currentLevel := l.GetLevel()
	if gl, ok := getConfigLogLevel(); ok {
		currentLevel = gl
	}

	if level < currentLevel {
		return
	}

	l.mu.RLock()
	out := l.output
	mod := l.module
	l.mu.RUnlock()

	entry := logEntry{
		timestamp: time.Now().Format("15:04:05.000"),
		level:     level.String(),
		module:    mod,
		message:   fmt.Sprintf(format, args...),
		output:    out,
		logger:    l,
	}

	// Non-blocking asynchronous dispatch to queue
	select {
	case logQueue <- entry:
		return
	default:
		// Queue saturated!
	}

	// For critical events (ERROR and CRITICAL), NEVER DROP!
	// Fall back to synchronous write under l.outMu to guarantee immediate, reliable delivery.
	if level >= LevelError {
		l.outMu.Lock()
		writeEntry(out, entry)
		l.outMu.Unlock()
		return
	}

	// For non-critical events (TRACE, DEBUG, INFO, WARN), track drop count explicitly
	atomic.AddUint64(&droppedLogCount, 1)
}

func (l *valhallaLogger) Debug(f string, a ...interface{})    { l.log(LevelDebug, f, a...) }
func (l *valhallaLogger) Info(f string, a ...interface{})     { l.log(LevelInfo, f, a...) }
func (l *valhallaLogger) Warn(f string, a ...interface{})     { l.log(LevelWarn, f, a...) }
func (l *valhallaLogger) Error(f string, a ...interface{})    { l.log(LevelError, f, a...) }
func (l *valhallaLogger) Trace(f string, a ...interface{})    { l.log(LevelTrace, f, a...) }
func (l *valhallaLogger) Critical(f string, a ...interface{}) { l.log(LevelCritical, f, a...) }

// Global delegated shortcuts
func Debug(f string, a ...interface{})    { GetGlobal().Debug(f, a...) }
func Info(f string, a ...interface{})     { GetGlobal().Info(f, a...) }
func Warn(f string, a ...interface{})     { GetGlobal().Warn(f, a...) }
func Error(f string, a ...interface{})    { GetGlobal().Error(f, a...) }
func Trace(f string, a ...interface{})    { GetGlobal().Trace(f, a...) }
func Critical(f string, a ...interface{}) { GetGlobal().Critical(f, a...) }
