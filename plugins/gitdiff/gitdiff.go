// Package gitdiff is the default provider for the split-pane Git diff view.
package gitdiff

import (
	"context"
	"os/exec"

	"github.com/Ceinl/plums/capabilities"
)

type Plugin struct{}

func (*Plugin) Name() string                      { return "git-diff" }
func (*Plugin) Init(capabilities.Host, any) error { return nil }

func (*Plugin) GitDiff(ctx context.Context, cwd string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--", ".")
	if cwd != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}
