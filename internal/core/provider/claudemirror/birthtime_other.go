//go:build !darwin

package claudemirror

import (
	"os"
	"time"
)

// birthTime has no portable implementation off macOS (Linux's Stat_t exposes no
// birth time), so callers fall back to the modification time.
func birthTime(os.FileInfo) (time.Time, bool) {
	return time.Time{}, false
}
