package app

import (
	"testing"

	publiclayout "github.com/Ceinl/plums/plums/layout"
)

func TestLayoutNodeFromPublic(t *testing.T) {
	node, err := LayoutNodeFromPublic(publiclayout.Row(
		publiclayout.Editor().Width("40%"),
		publiclayout.Column(
			publiclayout.Tabs().Height(1),
			publiclayout.Chat(),
		),
	))
	if err != nil {
		t.Fatalf("LayoutNodeFromPublic() error = %v", err)
	}
	if node.Type != "div" || node.Direction != "row" || len(node.Children) != 2 {
		t.Fatalf("root node = %+v", node)
	}
	if got := string(node.Children[0].Size.Width); got != `"40%"` {
		t.Fatalf("left width raw = %s, want quoted 40%%", got)
	}
	if node.Children[1].Direction != "column" || len(node.Children[1].Children) != 2 {
		t.Fatalf("right column = %+v", node.Children[1])
	}
	if got := string(node.Children[1].Children[0].Size.Height); got != `1` {
		t.Fatalf("tabs height raw = %s, want 1", got)
	}
}

func TestInstallPublicLayoutAddsLayoutAndMenu(t *testing.T) {
	cfg := &RenderConfig{
		Layouts: map[string]LayoutNode{},
		Menu:    []string{"chat"},
	}
	layout := publiclayout.Named("work", publiclayout.Column(publiclayout.Chat()))

	layoutType, err := InstallPublicLayout(cfg, layout)
	if err != nil {
		t.Fatalf("InstallPublicLayout() error = %v", err)
	}
	if layoutType != LayoutType("work") {
		t.Fatalf("layout type = %q, want work", layoutType)
	}
	if _, ok := cfg.Layouts["work"]; !ok {
		t.Fatal("public layout not installed")
	}
	if len(cfg.Menu) != 2 || cfg.Menu[1] != "work" {
		t.Fatalf("menu = %+v, want chat, work", cfg.Menu)
	}
}

func TestLayoutNodeFromPublicPreservesExtendedFields(t *testing.T) {
	pad := 2.0
	node, err := LayoutNodeFromPublic(
		publiclayout.Row(publiclayout.Editor()).
			MinWidth("MinSplitLayoutWidth").
			Fallback("chat").
			AlignItems("center").
			Padding(publiclayout.Padding{Left: &pad}).
			Style(publiclayout.Style{Background: []uint8{1, 2, 3}, Foreground: []uint8{4, 5, 6}}).
			WhenPopupOpen(publiclayout.Component("command_palette_panel")).
			Variants(map[string]string{"git_diff": "git_diff_log"}),
	)
	if err != nil {
		t.Fatalf("LayoutNodeFromPublic() error = %v", err)
	}
	if node.MinWidth != "MinSplitLayoutWidth" || node.Fallback != "chat" || node.AlignItems != "center" {
		t.Fatalf("node routing fields = %+v", node)
	}
	if node.Padding.Left == nil || *node.Padding.Left != 2 {
		t.Fatalf("left padding = %+v", node.Padding.Left)
	}
	if got := node.Style.Background; len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("background = %+v", got)
	}
	if node.Style.BackgroundToken != "" || node.Style.ForegroundToken != "" {
		t.Fatalf("unexpected tokens on raw style = %+v", node.Style)
	}
	if node.WhenPopupOpen == nil || node.WhenPopupOpen.Component != "command_palette_panel" {
		t.Fatalf("when_popup_open = %+v", node.WhenPopupOpen)
	}
	if node.Variants["git_diff"] != "git_diff_log" {
		t.Fatalf("variants = %+v", node.Variants)
	}
}

func TestLayoutNodeFromPublicPreservesThemeStyleTokens(t *testing.T) {
	node, err := LayoutNodeFromPublic(
		publiclayout.Editor().Style(publiclayout.ThemeStyle(publiclayout.ColorBgPanel, publiclayout.ColorText)),
	)
	if err != nil {
		t.Fatalf("LayoutNodeFromPublic() error = %v", err)
	}
	if node.Style.BackgroundToken != publiclayout.ColorBgPanel {
		t.Fatalf("background token = %q, want %q", node.Style.BackgroundToken, publiclayout.ColorBgPanel)
	}
	if node.Style.ForegroundToken != publiclayout.ColorText {
		t.Fatalf("foreground token = %q, want %q", node.Style.ForegroundToken, publiclayout.ColorText)
	}
	if len(node.Style.Background) != 0 || len(node.Style.Foreground) != 0 {
		t.Fatalf("raw style should be empty for token style: %+v", node.Style)
	}
}

func TestLayoutNodeFromPublicPreservesRawJSONSizes(t *testing.T) {
	layout, err := publiclayout.FromJSON("focus", []byte(`{
		"type": "div",
		"children": [
			{"component": "chat_output", "size": {"width": "100%", "height": "grow"}}
		]
	}`))
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}
	node, err := LayoutNodeFromPublic(layout.Tree())
	if err != nil {
		t.Fatalf("LayoutNodeFromPublic() error = %v", err)
	}
	if got := string(node.Children[0].Size.Width); got != `"100%"` {
		t.Fatalf("width raw = %s, want quoted 100%%", got)
	}
	if got := string(node.Children[0].Size.Height); got != `"grow"` {
		t.Fatalf("height raw = %s, want quoted grow", got)
	}
}
