package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// Level defines the logging level
type Level int

const (
	// Debug level for detailed troubleshooting
	Debug Level = iota
	// Info level for general operational information
	Info
	// Warn level for potentially harmful situations
	Warn
	// Error level for error conditions
	Error
	// Fatal level for critical errors that prevent operation
	Fatal
)

// String returns the string representation of the logging level
func (l Level) String() string {
	switch l {
	case Debug:
		return "debug"
	case Info:
		return "info"
	case Warn:
		return "warn"
	case Error:
		return "error"
	case Fatal:
		return "fatal"
	default:
		return "unknown"
	}
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Level     string                 `json:"level"`
	Timestamp string                 `json:"timestamp"`
	Message   string                 `json:"message"`
	FB        string                 `json:"fb_name,omitempty"`
	BatchID   string                 `json:"batch_id,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Fields    map[string]interface{} `json:"-"`
}

// Logger provides structured JSON logging for function blocks
type Logger struct {
	fbName string
	writer io.Writer
}

// NewLogger creates a new logger for the specified function block
func NewLogger(fbName string) *Logger {
	return &Logger{
		fbName: fbName,
		writer: os.Stdout,
	}
}

// WithWriter sets the writer for the logger
func (l *Logger) WithWriter(writer io.Writer) *Logger {
	l.writer = writer
	return l
}

// Log logs a message at the specified level
func (l *Logger) Log(level Level, msg string, fields map[string]interface{}) {
	entry := LogEntry{
		Level:     level.String(),
		Timestamp: time.Now().Format(time.RFC3339),
		Message:   msg,
		FB:        l.fbName,
		Fields:    fields,
	}

	// Extract special fields
	if batchID, ok := fields["batch_id"]; ok {
		entry.BatchID = fmt.Sprintf("%v", batchID)
		delete(fields, "batch_id")
	}

	if err, ok := fields["error"]; ok {
		entry.Error = fmt.Sprintf("%v", err)
		delete(fields, "error")
	}

	// Marshal the entry to JSON
	bytes, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling log entry: %v\n", err)
		return
	}

	// Marshal additional fields
	if len(fields) > 0 {
		// Remove closing brace
		bytes = bytes[:len(bytes)-1]

		// Add additional fields
		for k, v := range fields {
			bytes = append(bytes, fmt.Sprintf(",\"%s\":", k)...)
			valueBytes, err := json.Marshal(v)
			if err != nil {
				valueBytes = []byte("\"<marshaling error>\"")
			}
			bytes = append(bytes, valueBytes...)
		}

		// Add closing brace
		bytes = append(bytes, '}')
	}

	bytes = append(bytes, '\n')

	// Write the entry
	l.writer.Write(bytes)

	// Exit for fatal errors
	if level == Fatal {
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields map[string]interface{}) {
	l.Log(Debug, msg, fields)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields map[string]interface{}) {
	l.Log(Info, msg, fields)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields map[string]interface{}) {
	l.Log(Warn, msg, fields)
}

// Error logs an error message
func (l *Logger) Error(msg string, err error, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["error"] = err.Error()
	l.Log(Error, msg, fields)
}

// Fatal logs a fatal error message and exits
func (l *Logger) Fatal(msg string, err error, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["error"] = err.Error()
	l.Log(Fatal, msg, fields)
}

// WithBatch adds the batch_id field to the logger context
func (l *Logger) WithBatch(batchID string) *LoggerContext {
	return &LoggerContext{
		logger: l,
		fields: map[string]interface{}{
			"batch_id": batchID,
		},
	}
}

// WithFields adds the specified fields to the logger context
func (l *Logger) WithFields(fields map[string]interface{}) *LoggerContext {
	return &LoggerContext{
		logger: l,
		fields: fields,
	}
}

// LoggerContext provides a context for logging with predefined fields
type LoggerContext struct {
	logger *Logger
	fields map[string]interface{}
}

// Debug logs a debug message with the predefined fields
func (c *LoggerContext) Debug(msg string) {
	c.logger.Debug(msg, c.fields)
}

// Info logs an info message with the predefined fields
func (c *LoggerContext) Info(msg string) {
	c.logger.Info(msg, c.fields)
}

// Warn logs a warning message with the predefined fields
func (c *LoggerContext) Warn(msg string) {
	c.logger.Warn(msg, c.fields)
}

// Error logs an error message with the predefined fields
func (c *LoggerContext) Error(msg string, err error) {
	fields := make(map[string]interface{})
	for k, v := range c.fields {
		fields[k] = v
	}
	c.logger.Error(msg, err, fields)
}

// Fatal logs a fatal error message with the predefined fields and exits
func (c *LoggerContext) Fatal(msg string, err error) {
	fields := make(map[string]interface{})
	for k, v := range c.fields {
		fields[k] = v
	}
	c.logger.Fatal(msg, err, fields)
}

// WithField adds a field to the context
func (c *LoggerContext) WithField(key string, value interface{}) *LoggerContext {
	fields := make(map[string]interface{})
	for k, v := range c.fields {
		fields[k] = v
	}
	fields[key] = value
	return &LoggerContext{
		logger: c.logger,
		fields: fields,
	}
}
