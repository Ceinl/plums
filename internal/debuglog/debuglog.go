package debuglog

import (
	"log"
	"os"
	"sync"
)

var (
	once   sync.Once
	logger *log.Logger
	mu     sync.Mutex
	file   *os.File
)

// Close closes the debug log file if logging was enabled.
func Close() error {
	mu.Lock()
	defer mu.Unlock()

	if file == nil {
		return nil
	}

	err := file.Close()
	file = nil
	logger = nil
	return err
}

func Printf(format string, v ...any) {
	once.Do(func() {
		path := os.Getenv("PLUMS_LOG")
		if path == "" {
			return
		}

		mu.Lock()
		defer mu.Unlock()

		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return
		}
		file = f
		logger = log.New(f, "plums ", log.LstdFlags|log.Lmicroseconds)
	})

	mu.Lock()
	defer mu.Unlock()

	if logger != nil {
		logger.Printf(format, v...)
	}
}
