// Package sessions owns session commands for the default config.
package sessions

import (
	"context"

	"github.com/Ceinl/plums/capabilities"
)

type Plugin struct{}

func (*Plugin) Name() string                      { return "sessions" }
func (*Plugin) Init(capabilities.Host, any) error { return nil }

func (*Plugin) Commands() []capabilities.Command {
	return []capabilities.Command{
		slash("/new", "Create a fresh opencode session", func(_ context.Context, ctx capabilities.Ctx) error {
			ctx.Sessions().New()
			return nil
		}),
		slash("/sessions", "Open existing opencode sessions", func(_ context.Context, ctx capabilities.Ctx) error {
			ctx.Sessions().Picker()
			return nil
		}),
		{
			Name:   "Start new session",
			Detail: "Create a fresh opencode session",
			Do:     func(_ context.Context, ctx capabilities.Ctx) error { ctx.Sessions().New(); return nil },
		},
		{
			Name:   "Sessions list",
			Detail: "Open existing opencode sessions",
			Do:     func(_ context.Context, ctx capabilities.Ctx) error { ctx.Sessions().Picker(); return nil },
		},
	}
}

func slash(name, detail string, do func(context.Context, capabilities.Ctx) error) capabilities.Command {
	return capabilities.Command{
		Name:   name,
		Detail: detail,
		Do:     do,
		Title: func(capabilities.CommandState) capabilities.PaletteLabel {
			return capabilities.PaletteLabel{Title: name, Detail: detail, Disabled: true}
		},
	}
}
