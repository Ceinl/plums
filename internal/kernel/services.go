package kernel

import "github.com/Ceinl/plums/capabilities"

// CompletionRegistry is the host's implementation of capabilities.Completion. It
// is created once per Load and handed to plugins through Host.Services() at Init
// so they can contribute completion sources. Registration is last-wins per
// trigger (most-recent first), consistent with the registry merge rule. Core
// (internal/app) reads Loaded.Completion to drive the @file/slash dropdown and
// adds its own built-in sources.
type CompletionRegistry struct {
	sources []capabilities.CompletionSource
}

func newCompletionRegistry() *CompletionRegistry {
	return &CompletionRegistry{}
}

func (r *CompletionRegistry) Register(source capabilities.CompletionSource) {
	if source == nil {
		return
	}
	r.sources = append([]capabilities.CompletionSource{source}, r.sources...)
}

func (r *CompletionRegistry) Sources() []capabilities.CompletionSource {
	return append([]capabilities.CompletionSource(nil), r.sources...)
}

// hostServices is the Init-time (registration) view of capabilities.Services. Of
// the four services, only Completion is functional during plugin init —
// Palette/Clipboard/Selection require a live runtime context and so are no-op
// stubs here; their real implementations are returned by Core's runtime Ctx.
type hostServices struct {
	completion *CompletionRegistry
}

func (s hostServices) Completion() capabilities.Completion { return s.completion }
func (s hostServices) Palette() capabilities.Palette       { return noopPalette{} }
func (s hostServices) Clipboard() capabilities.Clipboard   { return noopClipboard{} }
func (s hostServices) Selection() capabilities.Selection   { return noopSelection{} }

type noopPalette struct{}

func (noopPalette) Open(string, []capabilities.ListItem, func(capabilities.ListItem)) {}
func (noopPalette) OpenCommandPalette()                                               {}

type noopClipboard struct{}

func (noopClipboard) Copy(string) error      { return nil }
func (noopClipboard) Paste() (string, error) { return "", nil }

type noopSelection struct{}

func (noopSelection) Current() string { return "" }
func (noopSelection) Copy() error     { return nil }
