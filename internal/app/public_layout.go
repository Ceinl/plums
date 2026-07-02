package app

import (
	"encoding/json"
	"fmt"

	"github.com/Ceinl/plums/capabilities"
	publiclayout "github.com/Ceinl/plums/plums/layout"
)

func InstallPublicLayout(cfg *RenderConfig, public capabilities.Layout) (LayoutType, error) {
	if cfg == nil {
		return "", fmt.Errorf("nil render config")
	}
	if public == nil {
		return "", fmt.Errorf("nil public layout")
	}
	name := public.Name()
	if name == "" {
		return "", fmt.Errorf("public layout has empty name")
	}
	node, err := LayoutNodeFromPublic(public.Tree())
	if err != nil {
		return "", fmt.Errorf("public layout %q: %w", name, err)
	}
	if cfg.Layouts == nil {
		cfg.Layouts = make(map[string]LayoutNode)
	}
	cfg.Layouts[name] = node
	if PublicLayoutSelectable(public) && !menuContains(cfg.Menu, name) {
		cfg.Menu = append(cfg.Menu, name)
	}
	return LayoutType(name), nil
}

func PublicLayoutSelectable(public capabilities.Layout) bool {
	if public == nil {
		return false
	}
	if visibility, ok := public.(capabilities.LayoutVisibility); ok {
		return visibility.Selectable()
	}
	return true
}

func LayoutNodeFromPublic(node capabilities.Node) (LayoutNode, error) {
	switch n := node.(type) {
	case nil:
		return LayoutNode{}, fmt.Errorf("nil node")
	case *publiclayout.Node:
		return layoutNodeFromPublicNode(n)
	case publiclayout.Node:
		return layoutNodeFromPublicNode(&n)
	default:
		return LayoutNode{}, fmt.Errorf("unsupported node type %T", node)
	}
}

func layoutNodeFromPublicNode(node *publiclayout.Node) (LayoutNode, error) {
	if node == nil {
		return LayoutNode{}, fmt.Errorf("nil node")
	}
	out := LayoutNode{
		Type:       node.Kind,
		Component:  node.ComponentName,
		Direction:  node.DirectionName,
		AlignItems: node.AlignItemsName,
		MinWidth:   node.MinWidthValue,
		Fallback:   node.FallbackName,
		Size: SizeNode{
			Width:  rawSize(node.WidthValue),
			Height: rawSize(node.HeightValue),
		},
		Padding: PaddingNode{
			Top:    node.PaddingValue.Top,
			Right:  node.PaddingValue.Right,
			Bottom: node.PaddingValue.Bottom,
			Left:   node.PaddingValue.Left,
		},
		Style: StyleNode{
			Background:      cloneUint8s(node.StyleValue.Background),
			Foreground:      cloneUint8s(node.StyleValue.Foreground),
			Muted:           cloneUint8s(node.StyleValue.Muted),
			Accent:          cloneUint8s(node.StyleValue.Accent),
			BackgroundToken: node.StyleValue.BackgroundToken,
			ForegroundToken: node.StyleValue.ForegroundToken,
			MutedToken:      node.StyleValue.MutedToken,
			AccentToken:     node.StyleValue.AccentToken,
		},
		Variants: cloneStringMap(node.VariantsMap),
	}
	if node.WhenPopupOpenNode != nil {
		replacement, err := LayoutNodeFromPublic(node.WhenPopupOpenNode)
		if err != nil {
			return LayoutNode{}, err
		}
		out.WhenPopupOpen = &replacement
	}
	for _, child := range node.ChildrenNodes {
		childNode, err := LayoutNodeFromPublic(child)
		if err != nil {
			return LayoutNode{}, err
		}
		out.Children = append(out.Children, childNode)
	}
	if out.Type == "" && len(out.Children) > 0 {
		out.Type = "div"
	}
	if out.Direction == "" && len(out.Children) > 0 {
		out.Direction = "column"
	}
	if out.Component == "" && len(out.Children) == 0 {
		return LayoutNode{}, fmt.Errorf("node has no component or children")
	}
	return out, nil
}

func rawSize(value any) json.RawMessage {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case json.RawMessage:
		return append(json.RawMessage(nil), v...)
	case []byte:
		return append(json.RawMessage(nil), v...)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return data
}

func cloneUint8s(value []uint8) []uint8 {
	if len(value) == 0 {
		return nil
	}
	return append([]uint8(nil), value...)
}

func cloneStringMap(value map[string]string) map[string]string {
	if len(value) == 0 {
		return nil
	}
	out := make(map[string]string, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

func menuContains(menu []string, name string) bool {
	for _, item := range menu {
		if item == name {
			return true
		}
	}
	return false
}
