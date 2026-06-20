package layouts

import "testing"

func TestPluginName(t *testing.T) {
	if name := (&Plugin{}).Name(); name != "ui/layouts" {
		t.Fatalf("Name() = %q, want ui/layouts", name)
	}
}

func TestPluginLayouts(t *testing.T) {
	got := (&Plugin{}).Layouts()
	names := make([]string, 0, len(got))
	for _, l := range got {
		names = append(names, l.Name())
	}
	// split / narrow_split intentionally ship as a user plugin, not here.
	want := []string{"chat", "zen", "narrow_chat"}
	if len(names) != len(want) {
		t.Fatalf("layout names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("layout names = %v, want %v", names, want)
		}
	}
}

func TestNarrowChatHidden(t *testing.T) {
	for _, l := range (&Plugin{}).Layouts() {
		selectable, ok := l.(interface{ Selectable() bool })
		hidden := ok && !selectable.Selectable()
		switch l.Name() {
		case "narrow_chat":
			if !hidden {
				t.Fatalf("narrow_chat should be hidden")
			}
		default:
			if hidden {
				t.Fatalf("%s should be selectable", l.Name())
			}
		}
	}
}
