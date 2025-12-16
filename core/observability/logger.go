package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// LogLevel represents the severity level of a log entry.
type LogLevel int

const (
	// LogLevelDebug represents debug-level logs.
	LogLevelDebug LogLevel = iota
	// LogLevelInfo represents info-level logs.
	LogLevelInfo
	// LogLevelWarn represents warning-level logs.
	LogLevelWarn
	// LogLevelError represents error-level logs.
	LogLevelError
)

// String returns the string representation of the log level.
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Fields represents key-value pairs for structured logging.
type Fields map[string]interface{}

// Logger provides structured logging capabilities.
type Logger struct {
	mu sync.RWMutex

	output   io.Writer
	minLevel LogLevel
	fields   Fields // Default fields to include in all logs
}

// Config holds configuration for the logger.
type LoggerConfig struct {
	Output   io.Writer // Output destination (default: os.Stderr)
	MinLevel LogLevel  // Minimum log level (default: LogLevelInfo)
	Fields   Fields    // Default fields to include in all logs
}

// DefaultLoggerConfig returns a default logger configuration.
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Output:   os.Stderr,
		MinLevel: LogLevelInfo,
		Fields:   make(Fields),
	}
}

// NewLogger creates a new structured logger.
func NewLogger(config LoggerConfig) *Logger {
	if config.Output == nil {
		config.Output = os.Stderr
	}
	return &Logger{
		output:   config.Output,
		minLevel: config.MinLevel,
		fields:   config.Fields,
	}
}

// WithFields returns a new logger with additional fields.
func (l *Logger) WithFields(fields Fields) *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	newFields := make(Fields)
	// Copy existing fields
	for k, v := range l.fields {
		newFields[k] = v
	}
	// Add new fields
	for k, v := range fields {
		newFields[k] = v
	}

	return &Logger{
		output:   l.output,
		minLevel: l.minLevel,
		fields:   newFields,
	}
}

// WithField returns a new logger with a single additional field.
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return l.WithFields(Fields{key: value})
}

// log writes a log entry.
func (l *Logger) log(level LogLevel, msg string, fields Fields) {
	if level < l.minLevel {
		return
	}

	// Merge default fields with provided fields
	allFields := make(Fields)
	for k, v := range l.fields {
		allFields[k] = v
	}
	for k, v := range fields {
		allFields[k] = v
	}

	// Create log entry
	entry := LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     level.String(),
		Message:   msg,
		Fields:    allFields,
	}

	// Serialize to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		// Fallback to simple format if JSON marshaling fails
		fmt.Fprintf(l.output, "[%s] %s: %s\n", level.String(), time.Now().Format(time.RFC3339), msg)
		return
	}

	l.mu.RLock()
	defer l.mu.RUnlock()
	fmt.Fprintln(l.output, string(data))
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fields[0]
	} else {
		f = make(Fields)
	}
	l.log(LogLevelDebug, msg, f)
}

// Info logs an info message.
func (l *Logger) Info(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fields[0]
	} else {
		f = make(Fields)
	}
	l.log(LogLevelInfo, msg, f)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fields[0]
	} else {
		f = make(Fields)
	}
	l.log(LogLevelWarn, msg, f)
}

// Error logs an error message.
func (l *Logger) Error(msg string, err error, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fields[0]
	} else {
		f = make(Fields)
	}
	if err != nil {
		f["error"] = err.Error()
	}
	l.log(LogLevelError, msg, f)
}

// LogEntry represents a structured log entry.
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// FromContext extracts logger from context, or returns default logger.
func FromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(loggerKey).(*Logger); ok {
		return logger
	}
	return DefaultLogger()
}

// WithLogger adds logger to context.
func WithLogger(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

type loggerContextKey string

const loggerKey loggerContextKey = "logger"

// DefaultLogger returns a default logger instance.
var defaultLogger *Logger
var defaultLoggerOnce sync.Once

func DefaultLogger() *Logger {
	defaultLoggerOnce.Do(func() {
		defaultLogger = NewLogger(DefaultLoggerConfig())
	})
	return defaultLogger
}

