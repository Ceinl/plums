// Package backend owns backend/model selection commands for the default config.
package backend

import (
	"context"

	"github.com/Ceinl/plums/capabilities"
)

type Plugin struct{}

func (*Plugin) Name() string                      { return "backend/commands" }
func (*Plugin) Init(capabilities.Host, any) error { return nil }

func (*Plugin) Commands() []capabilities.Command {
	return []capabilities.Command{
		slash("/backend", "Switch backend provider", func(_ context.Context, ctx capabilities.Ctx) error {
			ctx.Backends().Switch()
			return nil
		}),
		slash("/model", "Change the active model", func(_ context.Context, ctx capabilities.Ctx) error {
			ctx.Backends().ChangeModel()
			return nil
		}),
		{
			Name:   "Change model",
			Detail: "Select model for future prompts",
			Do:     func(_ context.Context, ctx capabilities.Ctx) error { ctx.Backends().ChangeModel(); return nil },
		},
		{
			Name:   "Backend provider",
			Detail: "Current backend: ",
			Do:     func(_ context.Context, ctx capabilities.Ctx) error { ctx.Backends().Switch(); return nil },
			Title: func(s capabilities.CommandState) capabilities.PaletteLabel {
				return capabilities.PaletteLabel{Title: "Backend provider", Detail: "Current backend: " + s.BackendProvider}
			},
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
