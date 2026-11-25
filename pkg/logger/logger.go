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
	"time"
)

// GlobalConfigAccessor define a interface necessária para o logger obter o nível global.
type GlobalConfigAccessor interface {
	GetLogLevel() int
}

var (
	configAccessorMu sync.RWMutex
	configAccessor   GlobalConfigAccessor
)

// SetConfigAccessor permite registrar um provedor de configuração global.
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

// LogLevel define a severidade da mensagem.
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

// ParseLogLevel converte uma string para o tipo LogLevel.
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

// Logger define a interface universal de observabilidade assíncrona do Valhalla.
type Logger interface {
	Critical(format string, args ...interface{})
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
	Trace(format string, args ...interface{})
	SetLevel(level LogLevel)
	GetLevel() LogLevel
}

// valhallaLogger é a implementação assíncrona desacoplada de alto desempenho do Logger.
type valhallaLogger struct {
	mu     sync.RWMutex
	level  LogLevel
	output io.Writer
	module string
}

type logEntry struct {
	timestamp string
	level     string
	module    string
	message   string
	output    io.Writer
	flushDone chan struct{}
}

var (
	globalLogger Logger
	once         sync.Once
	logQueue     = make(chan logEntry, 10000) // Buffer FIFO desacoplado
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
)

func init() {
	// Worker assíncrono em background para processamento de I/O de logs fora da hot path
	go func() {
		for entry := range logQueue {
			if entry.flushDone != nil {
				close(entry.flushDone)
				continue
			}

			out := entry.output
			if out == nil {
				out = os.Stdout
			}

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
				fmt.Fprintf(out, "[%s] %s[%s]%s [%s] %s\n",
					entry.timestamp, color, entry.level, colorReset, entry.module, entry.message)
			} else {
				fmt.Fprintf(out, "[%s] [%s] [%s] %s\n",
					entry.timestamp, entry.level, entry.module, entry.message)
			}
		}
	}()
}

// Flush bloqueia até que todas as mensagens enfileiradas até o momento sejam gravadas na saída.
func Flush() {
	done := make(chan struct{})
	select {
	case logQueue <- logEntry{flushDone: done}:
		<-done
	default:
	}
}

// InitGlobalLogger inicializa o logger global sem nome de módulo.
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

// NewLogger cria uma instância de logger para um módulo específico.
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

// GetGlobal retorna o logger global.
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

func (l *valhallaLogger) log(level LogLevel, format string, args ...interface{}) {
	currentLevel := l.GetLevel()
	if gl, ok := getConfigLogLevel(); ok {
		currentLevel = gl
	}

	if level < currentLevel {
		return
	}

	// Envio assíncrono não-bloqueante para a fila
	select {
	case logQueue <- logEntry{
		timestamp: time.Now().Format("15:04:05.000"),
		level:     level.String(),
		module:    l.module,
		message:   fmt.Sprintf(format, args...),
		output:    l.output,
	}:
	default:
		// Em caso de saturação do buffer de 10k, descarta para proteger latência de simulação
	}
}

func (l *valhallaLogger) Debug(f string, a ...interface{})    { l.log(LevelDebug, f, a...) }
func (l *valhallaLogger) Info(f string, a ...interface{})     { l.log(LevelInfo, f, a...) }
func (l *valhallaLogger) Warn(f string, a ...interface{})     { l.log(LevelWarn, f, a...) }
func (l *valhallaLogger) Error(f string, a ...interface{})    { l.log(LevelError, f, a...) }
func (l *valhallaLogger) Trace(f string, a ...interface{})    { l.log(LevelTrace, f, a...) }
func (l *valhallaLogger) Critical(f string, a ...interface{}) { l.log(LevelCritical, f, a...) }

// Atalhos globais delegados
func Debug(f string, a ...interface{})    { GetGlobal().Debug(f, a...) }
func Info(f string, a ...interface{})     { GetGlobal().Info(f, a...) }
func Warn(f string, a ...interface{})     { GetGlobal().Warn(f, a...) }
func Error(f string, a ...interface{})    { GetGlobal().Error(f, a...) }
func Trace(f string, a ...interface{})    { GetGlobal().Trace(f, a...) }
func Critical(f string, a ...interface{}) { GetGlobal().Critical(f, a...) }
