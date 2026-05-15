package debuglog

import (
	"log"
	"os"
	"sync"
)

var (
	once   sync.Once
	logger *log.Logger
)

func Printf(format string, v ...any) {
	once.Do(func() {
		path := os.Getenv("PLUMS_LOG")
		if path == "" {
			return
		}

		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return
		}
		logger = log.New(f, "plums ", log.LstdFlags|log.Lmicroseconds)
	})

	if logger != nil {
		logger.Printf(format, v...)
	}
}
