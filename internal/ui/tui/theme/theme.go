// Package theme is the single source of truth for the TUI colour palette.
// Components and layouts must take colours from here so every module stays
// visually consistent; raw RGB literals outside this package (and the user
// layout config) are a bug.
package theme

import "fmt"

type Color struct{ R, G, B uint8 }

// Fg returns the ANSI truecolor foreground escape for the colour.
func (c Color) Fg() string { return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B) }

// Bg returns the ANSI truecolor background escape for the colour.
func (c Color) Bg() string { return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B) }

// Backgrounds, darkest to lightest.
var (
	BgBackdrop  = Color{8, 7, 10}   // dimmed backdrop behind modal popups
	BgInput     = Color{20, 18, 26} // search / query fields
	BgBase      = Color{22, 20, 27} // main application background
	BgSurface   = Color{30, 27, 38} // popups, dropdowns, code blocks
	BgPanel     = Color{32, 30, 40} // editor pane
	BgRaised    = Color{34, 31, 43} // cards, user messages
	BgHighlight = Color{40, 38, 50} // current line, separator track
	BgSelected  = Color{48, 43, 61} // active row in lists / menus
)

// Text tiers, brightest to dimmest.
var (
	TextBright = Color{238, 234, 248} // active tabs, headings
	Text       = Color{232, 229, 241} // primary text
	TextSoft   = Color{200, 198, 212} // body copy, secondary text
	TextMuted  = Color{145, 140, 160} // labels, details, inactive items
	TextFaint  = Color{100, 98, 112}  // status lines, line numbers
	TextDim    = Color{72, 70, 84}    // rules, gutters
)

// Accents.
var (
	Accent       = Color{247, 184, 90} // primary accent (amber)
	AccentBold   = Color{220, 160, 50} // system / error headings
	AccentSoft   = Color{200, 145, 60} // system / error body
	BorderAccent = Color{152, 116, 66} // dim amber for highlighted panel borders
)

// Status indicators.
var (
	StatusReady    = Color{120, 130, 145}
	StatusStarting = Color{245, 190, 70}
	StatusThinking = Color{100, 190, 255}
	Success        = Color{167, 226, 185} // current session, tool success
)

// Selection, cursor and borders — shared by every text surface.
var (
	SelectionFg = Color{215, 225, 255}
	SelectionBg = Color{45, 80, 158}
	CursorFg    = Color{14, 14, 20}
	CursorBg    = Color{165, 188, 255}
	Border      = Color{80, 82, 96}
)

// Chat content colours.
var (
	ChatCursor     = Color{160, 220, 255} // streaming cursor
	ChatInlineCode = Color{245, 190, 120}
	ChatCodeFence  = Color{120, 118, 140}
	ChatListMarker = Color{155, 188, 255} // bullets / numbers
	ToolCall       = Color{115, 165, 135}
	ToolIndicator  = Color{175, 135, 90}
)

// Diff colours.
var (
	DiffFile     = Color{165, 205, 255}
	DiffHunk     = Color{180, 145, 245}
	DiffAddFg    = Color{125, 210, 150}
	DiffRemoveFg = Color{240, 130, 130}
	DiffAddBg    = Color{20, 36, 28}
	DiffRemoveBg = Color{42, 24, 30}
)

// Zen greys — neutral palette for the minimalistic "zen" layout. Kept in sync
// with the RGB literals in the public layouts plugin's "zen" node.
var (
	ZenBg   = Color{24, 24, 27} // unified background for chat output and input
	ZenText = Color{205, 205, 212}
)

// Unset is the zero colour produced by an empty layout.Style; components
// compare against Unset.Bg() to detect "no background set" and inherit.
var Unset = Color{}
