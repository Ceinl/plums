package theme

// Palette is the full set of swappable colours. The package-level colour vars
// in theme.go hold the *active* palette; Apply swaps them, Snapshot reads them,
// and Default captures the built-in set so a layout can override only the few
// colours it cares about instead of redefining everything.
type Palette struct {
	BgBackdrop, BgInput, BgBase, BgSurface, BgPanel, BgRaised, BgHighlight, BgSelected Color
	TextBright, Text, TextSoft, TextMuted, TextFaint, TextDim                          Color
	Accent, AccentBold, AccentSoft, BorderAccent                                       Color
	StatusReady, StatusStarting, StatusThinking, Success                               Color
	SelectionFg, SelectionBg, CursorFg, CursorBg, Border                               Color
	ChatCursor, ChatInlineCode, ChatCodeFence, ChatListMarker, ToolCall, ToolIndicator Color
	DiffFile, DiffHunk, DiffAddFg, DiffRemoveFg, DiffAddBg, DiffRemoveBg               Color
}

// Snapshot captures the currently active palette.
func Snapshot() Palette {
	return Palette{
		BgBackdrop: BgBackdrop, BgInput: BgInput, BgBase: BgBase, BgSurface: BgSurface,
		BgPanel: BgPanel, BgRaised: BgRaised, BgHighlight: BgHighlight, BgSelected: BgSelected,
		TextBright: TextBright, Text: Text, TextSoft: TextSoft, TextMuted: TextMuted,
		TextFaint: TextFaint, TextDim: TextDim,
		Accent: Accent, AccentBold: AccentBold, AccentSoft: AccentSoft, BorderAccent: BorderAccent,
		StatusReady: StatusReady, StatusStarting: StatusStarting, StatusThinking: StatusThinking, Success: Success,
		SelectionFg: SelectionFg, SelectionBg: SelectionBg, CursorFg: CursorFg, CursorBg: CursorBg, Border: Border,
		ChatCursor: ChatCursor, ChatInlineCode: ChatInlineCode, ChatCodeFence: ChatCodeFence,
		ChatListMarker: ChatListMarker, ToolCall: ToolCall, ToolIndicator: ToolIndicator,
		DiffFile: DiffFile, DiffHunk: DiffHunk, DiffAddFg: DiffAddFg, DiffRemoveFg: DiffRemoveFg,
		DiffAddBg: DiffAddBg, DiffRemoveBg: DiffRemoveBg,
	}
}

// Apply makes p the active palette. Components that cache resolved colour
// strings must be refreshed afterwards (see components.RefreshColors).
func Apply(p Palette) {
	BgBackdrop, BgInput, BgBase, BgSurface = p.BgBackdrop, p.BgInput, p.BgBase, p.BgSurface
	BgPanel, BgRaised, BgHighlight, BgSelected = p.BgPanel, p.BgRaised, p.BgHighlight, p.BgSelected
	TextBright, Text, TextSoft = p.TextBright, p.Text, p.TextSoft
	TextMuted, TextFaint, TextDim = p.TextMuted, p.TextFaint, p.TextDim
	Accent, AccentBold, AccentSoft, BorderAccent = p.Accent, p.AccentBold, p.AccentSoft, p.BorderAccent
	StatusReady, StatusStarting = p.StatusReady, p.StatusStarting
	StatusThinking, Success = p.StatusThinking, p.Success
	SelectionFg, SelectionBg = p.SelectionFg, p.SelectionBg
	CursorFg, CursorBg, Border = p.CursorFg, p.CursorBg, p.Border
	ChatCursor, ChatInlineCode, ChatCodeFence = p.ChatCursor, p.ChatInlineCode, p.ChatCodeFence
	ChatListMarker, ToolCall, ToolIndicator = p.ChatListMarker, p.ToolCall, p.ToolIndicator
	DiffFile, DiffHunk, DiffAddFg = p.DiffFile, p.DiffHunk, p.DiffAddFg
	DiffRemoveFg, DiffAddBg, DiffRemoveBg = p.DiffRemoveFg, p.DiffAddBg, p.DiffRemoveBg
}

// Default is the built-in palette, captured once at init from theme.go's
// literals. It is the global base every layout inherits unless it overrides.
var Default = Snapshot()

// Zen is the neutral-grey palette for the minimalistic zen layout: the built-in
// colours with the purple tint lifted out of the background family. Text,
// accents, status and chat/diff hues are inherited from Default unchanged.
var Zen = func() Palette {
	p := Default
	p.BgBackdrop = Color{8, 8, 9}
	p.BgInput = Color{26, 26, 29}
	p.BgBase = Color{24, 24, 27}
	p.BgSurface = Color{30, 30, 34}
	p.BgPanel = Color{32, 32, 36}
	p.BgRaised = Color{34, 34, 38}
	p.BgHighlight = Color{40, 40, 44}
	p.BgSelected = Color{44, 44, 50}
	p.Border = Color{70, 70, 78}
	return p
}()
