package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents a log level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

// String returns the string representation of a log level
func (l Level) String() string {
	switch l {
	case DEBUG:
		return "debug"
	case INFO:
		return "info"
	case WARN:
		return "warn"
	case ERROR:
		return "error"
	default:
		return "unknown"
	}
}

// ParseLevel converts a string to a Level
func ParseLevel(s string) (Level, error) {
	switch s {
	case "debug":
		return DEBUG, nil
	case "info":
		return INFO, nil
	case "warn":
		return WARN, nil
	case "error":
		return ERROR, nil
	default:
		return INFO, fmt.Errorf("invalid log level: %s", s)
	}
}

// Format represents the log output format
type Format int

const (
	JSON Format = iota
	TEXT
)

// ParseFormat converts a string to a Format
func ParseFormat(s string) (Format, error) {
	switch s {
	case "json":
		return JSON, nil
	case "text":
		return TEXT, nil
	default:
		return TEXT, fmt.Errorf("invalid log format: %s", s)
	}
}

// Logger provides structured logging
type Logger struct {
	level  Level
	format Format
	output io.Writer
	mu     sync.Mutex
}

// New creates a new logger
func New(levelStr, formatStr, outputStr string) (*Logger, error) {
	level, err := ParseLevel(levelStr)
	if err != nil {
		level = INFO
	}

	format, err := ParseFormat(formatStr)
	if err != nil {
		format = TEXT
	}

	var output io.Writer
	switch outputStr {
	case "stdout", "":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		// Treat as file path
		file, err := os.OpenFile(outputStr, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		output = file
	}

	return &Logger{
		level:  level,
		format: format,
		output: output,
	}, nil
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...interface{}) {
	if l.level <= DEBUG {
		l.log(DEBUG, msg, fields...)
	}
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...interface{}) {
	if l.level <= INFO {
		l.log(INFO, msg, fields...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...interface{}) {
	if l.level <= WARN {
		l.log(WARN, msg, fields...)
	}
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...interface{}) {
	if l.level <= ERROR {
		l.log(ERROR, msg, fields...)
	}
}

// log performs the actual logging
func (l *Logger) log(level Level, msg string, fields ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().UTC()

	if l.format == JSON {
		l.logJSON(timestamp, level, msg, fields...)
	} else {
		l.logText(timestamp, level, msg, fields...)
	}
}

// logJSON outputs in JSON format
func (l *Logger) logJSON(timestamp time.Time, level Level, msg string, fields ...interface{}) {
	entry := map[string]interface{}{
		"time":  timestamp.Format(time.RFC3339Nano),
		"level": level.String(),
		"msg":   msg,
	}

	// Add fields as key-value pairs
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key := fmt.Sprint(fields[i])
			entry[key] = fields[i+1]
		}
	}

	jsonData, err := json.Marshal(entry)
	if err != nil {
		// Fallback to simple output if JSON marshaling fails
		fmt.Fprintf(l.output, `{"time":"%s","level":"%s","msg":"json marshal error: %v"}`+"\n",
			timestamp.Format(time.RFC3339Nano), level.String(), err)
		return
	}

	fmt.Fprintln(l.output, string(jsonData))
}

// logText outputs in human-readable text format
func (l *Logger) logText(timestamp time.Time, level Level, msg string, fields ...interface{}) {
	fmt.Fprintf(l.output, "%s %s %s",
		timestamp.Format(time.RFC3339Nano),
		level.String(),
		msg)

	// Add fields as key=value pairs
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key := fmt.Sprint(fields[i])
			value := fmt.Sprint(fields[i+1])
			fmt.Fprintf(l.output, " %s=%s", key, value)
		}
	}

	fmt.Fprintln(l.output)
}

// SetLevel changes the log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// Close closes the underlying writer if it's a file
func (l *Logger) Close() error {
	if closer, ok := l.output.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
