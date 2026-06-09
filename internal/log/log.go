// Package log provides debug logging for codeg.
// When debug mode is enabled via main.go, all operations are logged.
package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu     sync.Mutex
	w      io.WriteCloser
	enabled bool
)

// Init opens the debug log file. Call from main.go when --debug is set.
func Init() {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".codeg")
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "codeg.log")

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	w = f
	enabled = true
	Info("=== codeg session start ===")
}

// Close closes the debug log.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if w != nil {
		Info("=== codeg session end ===")
		w.Close()
		w = nil
	}
	enabled = false
}

// Enabled returns true if debug logging is active.
func Enabled() bool { return enabled }

// Info logs a message with timestamp.
func Info(format string, args ...interface{}) {
	log("INFO", format, args...)
}

// Error logs an error message.
func Error(format string, args ...interface{}) {
	log("ERROR", format, args...)
}

func log(level, format string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	if w == nil {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(w, "%s [%s] %s\n", ts, level, msg)
}
