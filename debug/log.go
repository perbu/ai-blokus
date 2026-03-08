package debug

import (
	"fmt"
	"log"
	"os"
	"sync"
)

var (
	logger  *log.Logger
	logFile *os.File
	mu      sync.Mutex
	enabled bool
)

// Init opens the debug log file. Call Close() on shutdown.
func Init(path string) error {
	mu.Lock()
	defer mu.Unlock()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open debug log: %w", err)
	}
	logFile = f
	logger = log.New(f, "", log.Ltime|log.Lmicroseconds)
	enabled = true
	logger.Println("=== AI Blokus debug log started ===")
	return nil
}

// Close closes the log file.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
		enabled = false
	}
}

// Log writes a formatted message to the debug log.
func Log(format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if enabled {
		logger.Printf(format, args...)
	}
}
