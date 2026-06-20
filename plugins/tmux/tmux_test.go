package tmux

import (
	"testing"

	"github.com/Ceinl/plums/capabilities"
)

func TestPluginImplementsShutdownHook(t *testing.T) {
	var _ capabilities.Plugin = (*Plugin)(nil)
	var _ capabilities.OnShutdown = (*Plugin)(nil)
	if got := (&Plugin{}).Name(); got != "tmux" {
		t.Fatalf("Name() = %q, want tmux", got)
	}
}
