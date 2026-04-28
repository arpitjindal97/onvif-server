// Package logger provides simple leveled logging used across the project.
package logger

import "log"

// debugMode controls whether Debug messages are emitted.
var debugMode bool

// SetDebug enables or disables verbose debug logging.
func SetDebug(enabled bool) {
	debugMode = enabled
}

// Debug logs a message only when debug mode is enabled.
func Debug(format string, args ...interface{}) {
	if debugMode {
		log.Printf(format, args...)
	}
}

// Info logs an informational message; always shown.
func Info(format string, args ...interface{}) {
	log.Printf(format, args...)
}
