package claudemirror

import (
	"os"
	"syscall"
	"time"
)

// birthTime returns the file's creation (birth) time on macOS, where the
// underlying stat struct carries Birthtimespec.
func birthTime(info os.FileInfo) (time.Time, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(st.Birthtimespec.Sec, st.Birthtimespec.Nsec), true
}
