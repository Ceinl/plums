package layout

import "testing"

func TestSplitBuilder(t *testing.T) {
	l := Split(
		Editor().Width("40%"),
		Column(Tabs().Height(1), Chat(), StatusBar().Height(1)),
	)
	if l.Name() != "split" {
		t.Fatalf("layout name = %q, want split", l.Name())
	}
	root, ok := l.Tree().(*Node)
	if !ok {
		t.Fatalf("root type = %T, want *Node", l.Tree())
	}
	if root.DirectionName != "row" || len(root.ChildrenNodes) != 2 {
		t.Fatalf("root = %+v", root)
	}
	left, ok := root.ChildrenNodes[0].(*Node)
	if !ok || left.ComponentName != "editor" || left.WidthValue != "40%" {
		t.Fatalf("left node = %+v", root.ChildrenNodes[0])
	}
}

func TestFromJSONSingleNode(t *testing.T) {
	l, err := FromJSON("focus", []byte(`{
		"type": "div",
		"direction": "column",
		"children": [
			{"component": "chat_output", "size": {"height": "grow"}},
			{"component": "input_box", "size": {"height": 7}}
		]
	}`))
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}
	if l.Name() != "focus" {
		t.Fatalf("layout name = %q, want focus", l.Name())
	}
	root, ok := l.Tree().(*Node)
	if !ok {
		t.Fatalf("root type = %T, want *Node", l.Tree())
	}
	if root.DirectionName != "column" || len(root.ChildrenNodes) != 2 {
		t.Fatalf("root = %+v", root)
	}
	first := root.ChildrenNodes[0].(*Node)
	if first.ComponentName != "chat_output" {
		t.Fatalf("first component = %q", first.ComponentName)
	}
}

func TestFromJSONLayoutConfig(t *testing.T) {
	l, err := FromJSON("split", []byte(`{
		"version": 1,
		"layouts": {
			"split": {
				"type": "div",
				"direction": "row",
				"min_width": "MinSplitLayoutWidth",
				"fallback": "chat",
				"children": [
					{"component": "editor", "size": {"width": "40%"}},
					{"component": "chat_output"}
				]
			}
		}
	}`))
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}
	root := l.Tree().(*Node)
	if root.MinWidthValue != "MinSplitLayoutWidth" || root.FallbackName != "chat" {
		t.Fatalf("root min/fallback = %q/%q", root.MinWidthValue, root.FallbackName)
	}
}

func TestFromJSONSingleLayoutConfigInfersName(t *testing.T) {
	l, err := FromJSON("", []byte(`{
		"layouts": {
			"focus": {"component": "chat_output"}
		}
	}`))
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}
	if l.Name() != "focus" {
		t.Fatalf("layout name = %q, want focus", l.Name())
	}
}
