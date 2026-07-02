package app

import "github.com/Ceinl/plums/capabilities"

// completionRegistry is Core's implementation of capabilities.Completion. It is
// created once per app build and handed to plugins via Host at Init so they can
// contribute completion sources; the same registry is reachable at runtime
// through Ctx.Services().Completion(). Registration is last-wins per trigger,
// consistent with the kernel registry's merge rule.
type completionRegistry struct {
	sources []capabilities.CompletionSource
}

func newCompletionRegistry() *completionRegistry {
	return &completionRegistry{}
}

// buildCompletionRegistry assembles the runtime completion registry: it first
// registers the built-in @file and /slash sources bound to the live state, then
// layers the plugin-contributed sources on top (last-wins per trigger). This is
// the path that exercises the Completion capability contract for the built-ins.
func buildCompletionRegistry(state *State, contributed []capabilities.CompletionSource) *completionRegistry {
	registry := newCompletionRegistry()
	registry.Register(fileCompletionSource{paths: func() []string {
		if state.projectFiles == nil {
			state.projectFiles = projectFilePaths(".", 5000)
		}
		return state.projectFiles
	}})
	registry.Register(slashCompletionSource{commands: state.allSlashCommands})
	for _, source := range contributed {
		registry.Register(source)
	}
	return registry
}

func (r *completionRegistry) Register(source capabilities.CompletionSource) {
	if source == nil {
		return
	}
	// Prepend so the most-recent registration for a trigger wins lookups.
	r.sources = append([]capabilities.CompletionSource{source}, r.sources...)
}

func (r *completionRegistry) Sources() []capabilities.CompletionSource {
	return append([]capabilities.CompletionSource(nil), r.sources...)
}

// source returns the highest-priority registered source for the trigger rune, if
// any.
func (r *completionRegistry) source(trigger rune) capabilities.CompletionSource {
	for _, source := range r.sources {
		if source != nil && source.Trigger() == trigger {
			return source
		}
	}
	return nil
}

// --- Built-in completion sources ---

// fileCompletionSource models the existing @file completion behavior as a
// capabilities.CompletionSource. It is registered by Core at build time so the
// completion path is exercised through the service contract.
type fileCompletionSource struct {
	paths func() []string
}

func (s fileCompletionSource) Trigger() rune { return '@' }

func (s fileCompletionSource) Candidates(query string) []capabilities.Candidate {
	if s.paths == nil {
		return nil
	}
	query = normalizedQuery(query)
	items := make([]capabilities.Candidate, 0, 12)
	for _, path := range s.paths() {
		if query == "" || paletteMatches(query, path) {
			items = append(items, capabilities.Candidate{Value: path})
			if len(items) >= 12 {
				break
			}
		}
	}
	return items
}

// slashCompletionSource models the existing /slash completion behavior as a
// capabilities.CompletionSource.
type slashCompletionSource struct {
	commands func() []SlashCommand
}

func (s slashCompletionSource) Trigger() rune { return '/' }

func (s slashCompletionSource) Candidates(query string) []capabilities.Candidate {
	if s.commands == nil {
		return nil
	}
	input := "/" + query
	items := make([]capabilities.Candidate, 0)
	for _, command := range s.commands() {
		if hasPrefix(command.Name, input) {
			items = append(items, capabilities.Candidate{Value: command.Name, Detail: command.Detail})
		}
	}
	return items
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// services is Core's implementation of capabilities.Services. The Completion
// registry is shared for the life of the build (registration-time and runtime
// see the same sources); Palette, Clipboard, and Selection are bound to a live
// runtimeCtx and so are nil-safe no-ops when no runtime context is present (the
// Init-time Host path).
type services struct {
	completion *completionRegistry
	rt         *runtimeCtx
}

func (s services) Completion() capabilities.Completion { return s.completion }

func (s services) Palette() capabilities.Palette {
	return paletteService{rt: s.rt}
}

func (s services) Clipboard() capabilities.Clipboard {
	return clipboardService{rt: s.rt}
}

func (s services) Selection() capabilities.Selection {
	return selectionService{rt: s.rt}
}

// paletteService routes Palette calls through the existing runtime-list / command
// palette state mutations (state_palette.go), the same path Ctx.OpenList uses.
type paletteService struct {
	rt *runtimeCtx
}

func (p paletteService) Open(title string, items []capabilities.ListItem, onPick func(capabilities.ListItem)) {
	if p.rt == nil {
		return
	}
	items = append([]capabilities.ListItem(nil), items...)
	p.rt.enqueue(func(state *State) {
		state.SetRuntimeList(title, items, onPick)
	})
}

func (p paletteService) OpenCommandPalette() {
	if p.rt == nil {
		return
	}
	p.rt.enqueue(func(state *State) {
		state.OpenPalette()
	})
}

// clipboardService routes Copy through the existing writeClipboard path
// (clipboard.go) using the configured clipboard command.
type clipboardService struct {
	rt *runtimeCtx
}

func (c clipboardService) Copy(text string) error {
	cmd := ""
	if c.rt != nil {
		cmd = c.rt.clipboardCmd
	}
	return writeClipboard(text, cmd)
}

func (c clipboardService) Paste() (string, error) {
	return readClipboard()
}

// selectionService exposes the host-level selection→clipboard glue: the active
// selection (editor selection, else the focused component's SelectionProvider)
// and a Copy that writes it to the clipboard.
type selectionService struct {
	rt *runtimeCtx
}

func (s selectionService) Current() string {
	if s.rt == nil {
		return ""
	}
	return s.rt.selection
}

func (s selectionService) Copy() error {
	text := s.Current()
	if text == "" {
		return nil
	}
	return clipboardService{rt: s.rt}.Copy(text)
}
