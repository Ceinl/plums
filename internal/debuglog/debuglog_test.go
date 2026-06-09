package debuglog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrintfWritesToFileAndClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.log")
	t.Setenv("PLUMS_LOG", path)

	Printf("hello %s %d", "world", 42)
	Printf("second line")

	if err := Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "hello world 42") {
		t.Errorf("log missing formatted message, got: %q", content)
	}
	if !strings.Contains(content, "second line") {
		t.Errorf("log missing second message, got: %q", content)
	}
	if !strings.Contains(content, "plums ") {
		t.Errorf("log missing prefix, got: %q", content)
	}

	if err := Close(); err != nil {
		t.Errorf("second Close() should be nil, got: %v", err)
	}

	// After Close the logger is gone; Printf must not panic or recreate the file.
	Printf("after close")
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-reading log file: %v", err)
	}
	if strings.Contains(string(data), "after close") {
		t.Errorf("Printf after Close should not write, got: %q", string(data))
	}
}
