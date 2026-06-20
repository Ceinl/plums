// Package tmux owns terminal integration that tweaks tmux for the app run.
package tmux

import (
	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui"
)

type Plugin struct {
	keys *ui.TmuxKeys
}

func (*Plugin) Name() string { return "tmux" }

func (p *Plugin) Init(capabilities.Host, any) error {
	p.keys = ui.EnableTmuxExtendedKeys()
	return nil
}

func (p *Plugin) OnShutdown() {
	if p.keys != nil {
		p.keys.Restore()
	}
}
