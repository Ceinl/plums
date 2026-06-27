package plugintest

import (
	"context"
	"strings"
	"testing"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/config"
)

func TestCheckPluginAcceptsCommandPlugin(t *testing.T) {
	errs := CheckPlugin(config.Plugin{
		Self: &testPlugin{name: "demo"},
		Opts: testOptions{Message: "hi"},
	})
	if len(errs) != 0 {
		t.Fatalf("CheckPlugin() errors = %v", errs)
	}
}

func TestCheckPluginReportsBadCommand(t *testing.T) {
	errs := CheckPlugin(config.Plugin{Self: &testPlugin{name: "demo", badCommand: true}})
	if len(errs) == 0 {
		t.Fatal("expected errors")
	}
	got := joinErrors(errs)
	for _, want := range []string{"command", "nil Do"} {
		if !strings.Contains(got, want) {
			t.Fatalf("errors missing %q: %s", want, got)
		}
	}
}

func TestCheckPluginReportsComponentRenderPanic(t *testing.T) {
	errs := CheckPlugin(config.Plugin{Self: &componentPlugin{}})
	if len(errs) == 0 {
		t.Fatal("expected render smoke error")
	}
	if got := joinErrors(errs); !strings.Contains(got, "render smoke failed") {
		t.Fatalf("errors missing render smoke failure: %s", got)
	}
}

type testOptions struct {
	Message string
}

type testPlugin struct {
	name       string
	opts       testOptions
	badCommand bool
}

func (p *testPlugin) Name() string { return p.name }

func (p *testPlugin) Init(_ capabilities.Host, raw any) error {
	if opts, ok := raw.(testOptions); ok {
		p.opts = opts
	}
	return nil
}

func (p *testPlugin) Commands() []capabilities.Command {
	command := capabilities.Command{
		Name:   "demo.say",
		Detail: p.opts.Message,
		Do: func(context.Context, capabilities.Ctx) error {
			return nil
		},
	}
	if p.badCommand {
		command.Do = nil
	}
	return []capabilities.Command{command}
}

func joinErrors(errs []error) string {
	var b strings.Builder
	for _, err := range errs {
		b.WriteString(err.Error())
		b.WriteByte('\n')
	}
	return b.String()
}

type componentPlugin struct{}

func (*componentPlugin) Name() string                      { return "component" }
func (*componentPlugin) Init(capabilities.Host, any) error { return nil }
func (*componentPlugin) Components() []capabilities.Component {
	return []capabilities.Component{panicComponent{}}
}

type panicComponent struct{}

func (panicComponent) Name() string              { return "panic_component" }
func (panicComponent) Arrange(capabilities.Rect) {}
func (panicComponent) Render(capabilities.RenderCtx, capabilities.Surface) {
	panic("boom")
}
