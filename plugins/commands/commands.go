// Package commands is the built-in command plugin. It ships the standard plums
// command set — core slash commands (/new /command /backend /sessions /model)
// and the command-palette actions (change model, switch backend, new
// session, switch mode, layouts, thinking/tool-call visibility, output percent,
// sessions) — as a public, forkable CommandProvider plugin wired by the
// Default Config. Core itself ships no commands; this package defines them all,
// exactly like a user's own command plugin would.
//
// Every command drives the host through capabilities.Ctx verbs; dynamic palette
// titles are produced by the optional Command.Title func from Ctx.State().
package commands

import (
	"context"
	"strconv"

	"github.com/Ceinl/plums/capabilities"
)

// Plugin contributes the bundled command set.
type Plugin struct{}

func (*Plugin) Name() string                      { return "ui/commands" }
func (*Plugin) Init(capabilities.Host, any) error { return nil }

// Output-percent slider bounds, matching the legacy commands.json adjustment.
const (
	outputStep = 5
)

func (*Plugin) Commands() []capabilities.Command {
	return []capabilities.Command{
		// Keybind targets. These are real commands so the stock keymap and user
		// keymaps go through the same registry path, but they are hidden from the
		// command palette and slash dropdown.
		hidden("palette.open", "Open the command palette", func(_ context.Context, ctx capabilities.Ctx) error {
			ctx.OpenCommandPalette()
			return nil
		}),
		hidden("prompt.submit", "Submit the prompt", func(_ context.Context, ctx capabilities.Ctx) error {
			input := ctx.Input()
			if input != nil {
				ctx.Send(input.Text())
			}
			return nil
		}),

		// Slash commands. These appear in the editor's "/" dropdown and run their
		// Do when submitted. They are palette-hidden (Title returns disabled) so the
		// richer palette rows below are the ones shown in the command palette.
		slash("/new", "Create a fresh opencode session", func(_ context.Context, ctx capabilities.Ctx) error {
			ctx.NewSession()
			return nil
		}),
		slash("/command", "Open the command palette", func(_ context.Context, ctx capabilities.Ctx) error {
			ctx.OpenCommandPalette()
			return nil
		}),
		slash("/backend", "Switch backend provider", func(_ context.Context, ctx capabilities.Ctx) error {
			ctx.SwitchBackend()
			return nil
		}),
		slash("/sessions", "Open existing opencode sessions", func(_ context.Context, ctx capabilities.Ctx) error {
			ctx.OpenSessions()
			return nil
		}),
		slash("/model", "Change the active model", func(_ context.Context, ctx capabilities.Ctx) error {
			ctx.ChangeModel()
			return nil
		}),

		// Command-palette actions, in the legacy commands.json order.
		{
			Name:   "Change model",
			Detail: "Select model for future prompts",
			Do:     func(_ context.Context, ctx capabilities.Ctx) error { ctx.ChangeModel(); return nil },
		},
		{
			Name:   "Backend provider",
			Detail: "Current backend: ",
			Do:     func(_ context.Context, ctx capabilities.Ctx) error { ctx.SwitchBackend(); return nil },
			Title: func(s capabilities.CommandState) capabilities.PaletteLabel {
				return capabilities.PaletteLabel{Title: "Backend provider", Detail: "Current backend: " + s.BackendProvider}
			},
		},
		{
			Name:   "Start new session",
			Detail: "Create a fresh opencode session",
			Do:     func(_ context.Context, ctx capabilities.Ctx) error { ctx.NewSession(); return nil },
		},
		{
			Name:   "Switch mode",
			Detail: "Current mode: ",
			Do:     func(_ context.Context, ctx capabilities.Ctx) error { ctx.SwitchMode(); return nil },
			Title: func(s capabilities.CommandState) capabilities.PaletteLabel {
				title := "Switch mode"
				switch s.Mode {
				case "plan":
					title = "Switch to build mode"
				case "build":
					title = "Switch to plan mode"
				}
				return capabilities.PaletteLabel{Title: title, Detail: "Current mode: " + s.Mode}
			},
		},
		{
			Name:   "Layouts",
			Detail: "Select layout - current: ",
			Do:     func(_ context.Context, ctx capabilities.Ctx) error { ctx.SwitchLayout(); return nil },
			Title: func(s capabilities.CommandState) capabilities.PaletteLabel {
				return capabilities.PaletteLabel{Title: "Layouts", Detail: "Select layout - current: " + s.Layout}
			},
		},
		{
			Name:   "Thinking visibility",
			Detail: "",
			Do:     func(_ context.Context, ctx capabilities.Ctx) error { ctx.CycleThinkingVisibility(); return nil },
			Title: func(s capabilities.CommandState) capabilities.PaletteLabel {
				return capabilities.PaletteLabel{Title: "Thinking visibility", Detail: "Current: " + s.ThinkingVisibility + " (cycles full/title/hidden)"}
			},
		},
		{
			Name:   "Tool call visibility",
			Detail: "",
			Do:     func(_ context.Context, ctx capabilities.Ctx) error { ctx.CycleToolCallVisibility(); return nil },
			Title: func(s capabilities.CommandState) capabilities.PaletteLabel {
				return capabilities.PaletteLabel{Title: "Tool call visibility", Detail: "Current: " + s.ToolCallVisibility + " (cycles full/collapse/hidden)"}
			},
		},
		{
			Name:   "Output percentage",
			Detail: "",
			Do:     func(_ context.Context, ctx capabilities.Ctx) error { return nil },
			Title: func(s capabilities.CommandState) capabilities.PaletteLabel {
				return capabilities.PaletteLabel{
					Title:  "Output percentage",
					Detail: "Left/Right adjust - current: " + strconv.Itoa(s.OutputPercent) + "%",
					Adjust: true,
					Step:   outputStep,
				}
			},
		},
		{
			Name:   "Sessions list",
			Detail: "Open existing opencode sessions",
			Do:     func(_ context.Context, ctx capabilities.Ctx) error { ctx.OpenSessions(); return nil },
		},
	}
}

// hidden builds a command that can be run by name (usually from a keybind) but
// does not appear in the command palette.
func hidden(name, detail string, do func(context.Context, capabilities.Ctx) error) capabilities.Command {
	return capabilities.Command{
		Name:   name,
		Detail: detail,
		Do:     do,
		Title: func(capabilities.CommandState) capabilities.PaletteLabel {
			return capabilities.PaletteLabel{Title: name, Detail: detail, Disabled: true}
		},
	}
}

// slash builds a slash command that is hidden from the command-palette view (its
// dynamic Title is disabled) but surfaces in the editor "/" dropdown by Name.
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
