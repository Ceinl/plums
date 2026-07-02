# plums — Plugin-First Architecture Design

> Status: **design draft**. This captures the target architecture for plums' evolution
> into a deeply configurable, plugin-first AI agent TUI — "the neovim of AI UIs."
> It is a decision record to revise and build against, not a finished spec.

## 1. Vision

plums should be configurable the way neovim is configurable: not a fixed app with
options, but a small **kernel** plus a stack of **plugins** — where the built-in
behavior is itself made of plugins, and any of it can be replaced or extended.

The ceiling for configuration is the whole Go language. Config is a compiled Go
program (the xmonad / suckless model), not a DSL and not a constrained data format.

## 2. Principles

1. **Everything is a plugin.** Components, layouts, backends, commands, and even the
   defaults ship as a bundled `core` plugin at lowest priority. The kernel itself
   does almost nothing.
2. **Small required floor, infinite ceiling.** A plugin's *required* interface is
   tiny; capabilities are **optional interfaces** discovered by type assertion
   (the idiom already used in `palette.go`/`run.go`: `x.(sessionResetter)`).
3. **Defaults are the lowest-priority plugin.** Overriding a built-in means
   registering something with the same name later — **last-wins**.
4. **Dogfooding enforces completeness.** If a built-in can't be expressed against the
   public API, the API is too weak. There is no privileged internal path.
5. **Config is the single source of truth** for what is loaded and how it is wired.
6. **Keep the concepts, rewrite the realisation.** The backend/provider *abstraction*
   and the TUI *component model* are good and survive; their current implementations
   are ported behind the new interfaces rather than preserved verbatim.

## 3. Architecture overview

```
┌─────────────────────────────────────────────────────────────┐
│ user config.go  (compiled in; the root; imports plugins)     │
├─────────────────────────────────────────────────────────────┤
│ plugins:  core (bundled defaults)  +  3rd-party / user       │
│   • components   • layouts   • backends   • commands • hooks  │
├─────────────────────────────────────────────────────────────┤
│ public API  (package `plums`)                                │
│   Plugin · Config · Host · Ctx · RenderCtx · capability ifaces│
├─────────────────────────────────────────────────────────────┤
│ KERNEL (irreducible, not a plugin):                          │
│   registry · event loop + bus · render driver · state store  │
└─────────────────────────────────────────────────────────────┘
```

## 4. The kernel

The kernel is the only code that is *not* a plugin. It should stay small:

- **Registry** — `(kind, name) → Command / Component / Layout / Backend`. Last-wins,
  with a defined load order and shadow-logging (§7).
- **Event loop + bus** — pumps keyboard input and backend stream events; dispatches
  optional hook interfaces (`OnMessage`, `OnToolCall`, `OnSessionStart`).
- **Render driver** — owns the terminal, assigns rects to components per the active
  layout, and calls `Arrange(Rect)` / `Render(RenderCtx, Surface)`.
- **State store** — the single canonical state; the only thing components read,
  through `RenderCtx` (§6).
- **Public API surface** — the `plums` package types below.

If the kernel grows beyond this list, something replaceable has leaked into it.

## 5. Plugin model

### 5.1 Config — the root (narrow floor)

Config is declarative and evaluated once at startup. It is given **no `Host`** — its
"basic availability" is to declare settings and list plugins. It may *optionally*
opt into the same capability interfaces plugins use (e.g. a quick `/command` without
authoring a plugin), but is never required to.

```go
package plums

// The root. The user's config.go provides exactly one via plums.Use(...).
type Config interface {
    Settings() Settings   // typed replacement for config.toml + layout.json
    Plugins()  []Plugin   // which plugins to activate, and their options
}

type Settings struct {
    Backend      string     // name of the active backend plugin
    Layout       Layout     // default layout (Go builder or layout.FromJSON)
    Theme        Theme
    Keybinds     []Keybind
    HideThinking bool
    Disable      []RegistryKey // registry entries to remove (see §7)
    // ...the rest of today's config.toml/layout.json surface, typed
}

type RegistryKind string

const (
    RegistryCommand   RegistryKind = "command"
    RegistryComponent RegistryKind = "component"
    RegistryLayout    RegistryKind = "layout"
    RegistryBackend   RegistryKind = "backend"
    RegistryHook      RegistryKind = "hook"
)

type RegistryKey struct {
    Kind RegistryKind
    Name string
}
```

### 5.2 Plugin + optional capabilities

```go
type Plugin interface {
    Name() string
    Init(Host) error
}

// Optional capabilities — Config OR Plugin may implement any of these.
// The kernel discovers them by type assertion.
type CommandProvider   interface { Commands() []Command }
type ComponentProvider interface { Components() []Component }
type LayoutProvider    interface { Layouts() []Layout }
type BackendProvider   interface { Backends() []BackendRegistration }

// Event hooks — also optional interfaces.
type OnMessage     interface { OnMessage(Ctx, Message) }
type OnSessionStart interface { OnSessionStart(Ctx, Session) }
type OnToolCall    interface { OnToolCall(Ctx, ToolCall) }

// Input/render capabilities (KeyHandler, MouseHandler, SelectionProvider,
// DirtyTracker) are component-level, not plugin-level — see §5.5. A non-component
// plugin that wants a global key binds it via Settings.Keybinds → command rather
// than implementing KeyHandler here.
```

Activation is uniform for config and plugins:

```go
func activate(p any, host Host) {
    if pl, ok := p.(Plugin); ok { _ = pl.Init(host) }
    if cp, ok := p.(CommandProvider);   ok { host.addCommands(cp.Commands()) }
    if cm, ok := p.(ComponentProvider); ok { host.addComponents(cm.Components()) }
    if lp, ok := p.(LayoutProvider);    ok { host.addLayouts(lp.Layouts()) }
    if bp, ok := p.(BackendProvider);   ok { host.addBackends(bp.Backends()) }
    if mo, ok := p.(OnMessage);         ok { host.onMessage(mo) }
    // new capability later = new interface; existing plugins keep compiling.
}
```

### 5.3 Host vs Ctx

```go
// Host: setup-time (Init). Read settings, log, register.
type Host interface {
    Settings() Settings
    Log(format string, args ...any)
}

// Ctx: run-time (commands + hooks). Act on the live session.
type Ctx interface {
    Session() Session
    Input() Editor                                     // prompt buffer
    Selection() string
    Send(text string)                                  // submit a prompt
    Chat(role, text string)                            // append to the log
    Shell(context.Context, string, ...string) (string, error) // cwd + timeout handled
    SetLayout(name string)
    OpenList(title string, items []ListItem, onPick func(ListItem))
}
```

**Thread-safety:** `Ctx` methods are safe to call from command goroutines. Commands run
off the event loop (§9), so the kernel marshals all `Ctx` mutations (`Send`, `Chat`,
`SetLayout`, `OpenList`, …) back onto the event loop before they touch the state store.
Plugin authors never lock or touch the store directly — without this rule they would
race it.

### 5.4 Backends as plugins — *keep the concept, rewrite the realisation*

The current `adapter.Backend` abstraction is good and is preserved almost verbatim
as the public `Backend` capability. The data model (`Session`, `Provider`, `Model`,
`StreamEvent`, `ToolEvent`, `QuestionRequest`) carries over. What changes:

- A backend is delivered by a `BackendProvider` plugin and registered **by name**, so
  last-wins lets users swap or shadow the bundled ones.
- Backend registration includes runtime metadata and lifecycle hooks. The backend
  interface itself remains the protocol port; startup/managed-process concerns are
  capabilities of a registered runtime, not generic adapter methods.
- The *implementations* (opencode server lifecycle, codex protocol, claude-code
  stream-json, claude-mirror) get reimplemented cleanly as plugins rather than kept
  as-is — the realisation is what we're unhappy with, not the contract.

#### Three-layer backend stack (anti-corruption layer)

Today every provider package imports `adapter` and implements `adapter.Backend`
directly, fusing raw protocol code with domain translation. Provider concerns have
even leaked *up* into the generic layer (`adapter.DefaultBaseURL`,
`adapter.DefaultPortForDir` are opencode-specific). Fix this by inserting a per-provider
**bridge** (anti-corruption layer) between the raw client and the generic port:

```
opencode  →  backend/opencode  →  plums.Backend  →  user/kernel
(raw SDK)    (bridge / ACL)       (public port)
```

| Layer | Owns | Imports | Knows |
|---|---|---|---|
| `provider/opencode` (raw) | HTTP/SSE, native types, server lifecycle, per-dir port | stdlib only | only opencode |
| `backend/opencode` (bridge) | native→domain translation; implements `plums.Backend` | raw client **and** public API | both |
| public backend port | `Backend` iface + domain types + shared helpers | nothing provider-specific | nothing |
| kernel | uses backends | public API only | nothing |

**Discipline — dependencies point inward:** the raw client must not import the public
backend port; the port must not import any provider; *all* foreign-model translation
lives only in the bridge. Consequences:

- The raw `opencode` package becomes a standalone, dependency-free SDK (independently
  testable/reusable).
- `DefaultBaseURL` / `DefaultPortForDir` move out of the shared port into the opencode
  layer where they belong — the port stops being a dumping ground.
- Provider quirks (codex protocol, claude stream-json, claude-mirror attach-not-create)
  stay isolated in their bridge; the port never grows a provider-specific branch.

The plugin and ACL boundaries align: a `BackendProvider` plugin = raw client + bridge,
exposing a `BackendRegistration`. Optional backend capabilities (`ServerProcess`,
`ResetSession`, `AbortSession`) remain optional interfaces on the bridge or runtime.

#### Prior art: t3code (pingdotgg) — adopt ACP, tag event provenance

`pingdotgg/t3code` (a multi-agent GUI for Claude/Codex/OpenCode) independently arrived
at this exact three-layer split, and adds two ideas worth adopting:

- **ACP as a first-class backend type.** t3code has a generated, standalone
  `effect-acp` package implementing Zed's **Agent Client Protocol** (JSON-RPC over
  stdio), treated as *one backend among several* — Codex, which doesn't speak ACP, gets
  its own bespoke `effect-codex-app-server` package instead. **Decision for plums: add
  an `acp` backend plugin.** Any ACP-speaking agent (Claude Code, Gemini, Zed) is then
  supported by one bridge; only non-ACP agents (codex app-server, opencode SDK) need a
  bespoke bridge. ACP is where the ecosystem is converging.
- **Normalize but keep provenance.** t3code's `RuntimeEventRaw` carries a `source`
  discriminator (`opencode.sdk.event`, `codex.app-server.notification`, `claude.sdk.*`,
  `acp.jsonrpc`, `acp.{ext}.extension`). Events are normalized to the domain model but
  tagged with where they came from. **Decision for plums: add an optional `Source` field
  to `StreamEvent`** — near-free, and it powers `plums doctor`, debugging, and
  provider-specific extension handling by plugins.
- Considered and deferred: **schema-generated raw clients** (t3code generates protocol
  types from each agent's schema — drift-resistant but tooling-heavy; revisit for ACP),
  and a **provider / instance / runtime three-tier** model (plums already has
  backend + runtime; "instance" = a configured/authed connection — adopt only if
  multi-instance becomes a real need; see Open Question #6). Effect's DI Layers are
  TS-specific; the plugin `Host` is the Go-idiomatic equivalent.

```go
// Public, preserved from internal/core/adapter.Backend.
// No Name() — backend identity lives in BackendRegistration.Name (the registry key).
// If a diagnostic name is ever added back, it must not drive registry identity.
type Backend interface {
    Health(ctx context.Context) error
    CreateSession(ctx context.Context, dir string) (*Session, error)
    ListSessions(ctx context.Context) ([]Session, error)
    GetSession(ctx context.Context, id string) (*Session, error)
    ListMessages(ctx context.Context, id string) ([]MessageResponse, error)
    ListProviders(ctx context.Context) ([]Provider, []string, error)
    SendMessageEvents(ctx context.Context, id, text, providerID, modelID, agent string) <-chan StreamEvent
    ReplyQuestion(ctx context.Context, requestID string, answers [][]string) error
}

type BackendRegistration struct {
    Name    string // the registry key — single source of truth for backend identity
    Label   string
    Backend Backend
    // Startup is a first-class field because nearly every backend needs lifecycle
    // setup; rarer concerns (ResetSession, AbortSession) stay optional interfaces.
    // nil Startup means the backend is immediately usable after registration.
    Startup func(context.Context, Backend) (*StartupResult, error)
}

type StartupResult struct {
    Session *Session
    Server  ServerProcess
}

type ServerProcess interface {
    Stop()
    Done() <-chan struct{}
}

// Optional backend capabilities stay optional interfaces, as today:
//   ResetSession (claude-mirror), AbortSession (opencode), etc.
```

### 5.5 Components and layouts as built-in plugins

The current TUI system (the `div` layout engine, named components, the render loop)
is good and is **adapted**, not discarded. The adaptation:

- Built-in components (`chat_output`, `input_box`, `editor`, `sessions`, status bars)
  and built-in layouts (`split`, `chat`, `zen`, `fullscreen`) are registered by the
  `core` plugin, **by name**, exactly like third-party ones.
- `layout.json` already addresses components by string name — that registry *is* the
  override seam. It keeps working; the names it references are now replaceable.
- Components and layouts are orthogonal: override `chat_output` to change a pane's
  rendering, override `split` to rearrange panes — independently.

```go
type Component interface {
    Name() string
    Arrange(Rect)   // assigned its rect before paint (renamed from Layout to avoid
                    // clashing with the Layout type below)
    Render(RenderCtx, Surface)
}

// Optional component capabilities — the canonical home for input/selection/dirty.
// The common component contract stays small; expensive or interactive behaviour is
// discovered only when present.
type KeyHandler        interface { HandleKey(Ctx, KeyEvent) bool }
type MouseHandler      interface { HandleMouse(Ctx, MouseEvent) bool }
type SelectionProvider interface { Selection() string }
type DirtyTracker      interface { IsDirty() bool; ClearDirty() }

type Layout interface {
    Name() string
    Tree() Node   // div tree referencing components by name; or built programmatically
}
```

**Input dispatch order (reserved):** key and mouse events route in a fixed order —

```
overlay / focused component → active layout components → global keybinds → editor fallback
```

A handler returning `true` consumes the event and stops propagation. Fixing this order
now keeps it stable before component `KeyHandler` / `MouseHandler` implementations harden.

## 6. State model

Decision: **central store + read-only views, with an event bus layered on top for
hooks.** Rationale: lowest-risk path that still gives plugins everything, and it lets
the existing `*State`-coupled components be ported with minimal reshaping. (MVU /
bubbletea-style was considered and rejected for v1 as too large a paradigm shift;
revisit only if central-store fights plugin composability.)

- The kernel owns canonical state (conversation, session, streaming status, etc.).
- Components **read** it only through `RenderCtx` — never a fat struct.
- Mutations go through `Ctx` (`Send`, `Chat`, `SetLayout`, …).
- Hooks (`OnMessage`, `OnToolCall`) are the event bus — already part of the plugin API.
- Components may own **ephemeral local UI state**: cursor position, scroll/cache data,
  mouse selection, dirty bits, measured line caches. Canonical application state stays
  in the store; local render/input state stays with the component instance. This keeps
  `RenderCtx` from becoming a dumping ground and preserves current performance tricks.

```go
// The ONLY read path into state for a component.
type RenderCtx interface {
    Rect()  Rect
    Theme() Theme
    Messages()  []Message
    Streaming() bool
    Session()   Session
    Input()     EditorView
    // accretes (read-only) as built-ins demand more — that accretion IS the API
    // being driven to completeness by its own defaults.
}
```

**Discipline:** `RenderCtx` is the only way *any* component reads state, including
built-ins. When a built-in needs something it lacks, add a read accessor to
`RenderCtx` rather than reaching into kernel internals.

## 7. Registry, override, and introspection

### Last-wins + load order

Last-wins enables override ("replace defaults and parts of plugins you don't need").
To make it predictable, the load order is fixed — *later overrides earlier; the user
always wins*:

```
1. core plugin            (bundled defaults — lowest priority)
2. Config.Plugins()       (in slice order)
3. Config's own capabilities  (highest — always wins)
```

### Replace vs remove

Last-wins **replaces** a name; it does not delete. Removing a built-in command/pane
you don't want is an explicit gesture: `Settings.Disable []RegistryKey` (qualified
registry entries to drop after load). Clearer than registering an empty sentinel and
avoids collisions between `command.prs`, `component.prs`, `layout.prs`, etc.

### Introspection (required, not polish)

Silent shadowing in a last-wins world is a debugging trap. Two safeguards:

- The kernel **logs every shadowed registration** (`core.chat_output overridden by config`).
- A `plums doctor` command prints the **resolved** registry with provenance — the
  neovim `:checkhealth` / `:verbose` analog.

## 8. Build & distribution model

The compiled (xmonad) model:

```
~/.config/plums/
  config.go               # the root; plums.Use(&Config{}); imports its plugins
  plugins/
    github/github.go
    cost-tracker/...
```

- `config.go` **imports** the plugins it uses and lists them in `Plugins()`, so the
  build transitively pulls them in. The single source of truth for "what's loaded"
  is `Config.Plugins()`.
- **Plugin config = Go constructor args** — fully typed, no separate plugin-settings
  format: `github.New(github.Options{Limit: 20})`.
- `plums build` (a.k.a. `--recompile`) compiles `config.go` into a personalized
  binary, then runs it. No runtime `.so` loading (the stdlib `plugin` route is
  version-locked and platform-limited — explicitly rejected).
- Plugins are **not** hot-reloadable, by decision. Reconfiguring = recompile.
- Optional later sugar: folder-scan + blank-import codegen for drop-in plugins that
  self-register via `init()`. Not foundational.

### Build mechanics

`plums build` creates a temporary Go module rather than compiling the config file in
place. The generated module contains a small `main.go` that imports the user's
`config.go` package and the plums public API, then calls the kernel entry point.

Local plugins under `~/.config/plums/plugins/...` are addressed through a generated
`replace` directive — e.g. `replace github.com/Ceinl/plums-user => ~/.config/plums` —
so config and its local plugins share one stable module path
(`github.com/Ceinl/plums-user`) rather than invalid ad-hoc paths.
Remote plugins are normal Go module dependencies and are pinned by the generated
`go.mod` / `go.sum`. The compiled binary is cached by a hash of config sources,
plugin sources, `go.mod`, and the plums version; cache misses rebuild, cache hits run
immediately.

This keeps the user-facing model simple (`config.go` imports plugins), while making
imports deterministic and avoiding invalid ad-hoc paths.

### Worked example — config

```go
// ~/.config/plums/config.go — an importable package, NOT package main.
// `plums build` generates the main.go that imports this package and calls plums.Run();
// plums.Use(...) in init registers this config for that generated entry point.
// Config and local plugins share one stable module path (github.com/Ceinl/plums-user);
// plums build emits `replace github.com/Ceinl/plums-user => ~/.config/plums`.
package config

import (
    "github.com/Ceinl/plums/plums"
    "github.com/Ceinl/plums/plums/layout"
    "github.com/Ceinl/plums-user/plugins/github"
)

func init() { plums.Use(&Config{}) }

type Config struct{}

func (Config) Settings() plums.Settings {
    return plums.Settings{
        Backend: "opencode",
        Layout: layout.Split(
            layout.Editor().Width("40%"),
            layout.Column(layout.Tabs(), layout.Chat(), layout.StatusBar()),
        ),
        Keybinds: []plums.Keybind{{Key: "ctrl+p", Do: "open_palette"}},
    }
}

func (Config) Plugins() []plums.Plugin {
    return []plums.Plugin{ github.New(github.Options{Limit: 20}) }
}
```

### Worked example — a `/prs` plugin

```go
// ~/.config/plums/plugins/github/github.go
package github

import (
    "context"
    "fmt"
    "github.com/Ceinl/plums/plums"
)

type Options struct{ Limit int }
func New(o Options) *Plugin { return &Plugin{o} }

type Plugin struct{ o Options }

func (p *Plugin) Name() string          { return "github" }
func (p *Plugin) Init(plums.Host) error { return nil }

func (p *Plugin) Commands() []plums.Command {
    return []plums.Command{{
        Name:   "/prs",
        Detail: "List open pull requests",
        Do: func(ctx context.Context, c plums.Ctx) error {
            out, err := c.Shell(ctx, "gh", "pr", "list", "--limit", fmt.Sprint(p.o.Limit))
            if err != nil { return err }
            c.Chat("system", out)
            return nil
        },
    }}
}
```

## 9. Migration plan (strangler, not big-bang)

"From scratch" applies to the **kernel and public API**. Proven internals are
**ported** behind the new interfaces, not rewritten from zero — this avoids
re-discovering solved edge cases (chatlog freeze guard, narrow-layout fallbacks,
per-keystroke save, streaming quirks across backends).

Suggested order:

1. **Kernel + public `plums` package skeleton** — registry, load order, `Host`/`Ctx`,
   capability interfaces. No behavior yet.
2. **Backend registration + lifecycle model** — name/label/backend/startup, optional
   runtime capabilities, and one provider moved through the raw-client → bridge split.
   This validates the anti-corruption boundary before more providers are ported.
3. **`chat_output` as the first component plugin** — the hardest one (streaming,
   scroll, markdown, tool-call folding, selection). Porting it *first* validates that
   `RenderCtx` is expressive enough before everything else depends on it. The existing
   `chatlog.go` logic is rehomed largely intact.
4. **Remaining components + layouts** into the `core` plugin.
5. **Remaining backends** reimplemented as `BackendProvider` plugins, preserving the
   `Backend` contract and data model.
6. **Commands/palette/keybinds** onto the new command + capability model. Commands run
   off the event loop with `context.Context`; results re-enter through `Ctx`/bus calls.
7. **`plums build`** tooling and the compiled-config flow.
8. **`plums doctor`** introspection.

Each step should leave a runnable app (strangler), never a months-long broken valley.

## 10. Reserved seams

- The harness agent prompts (`internal/core/harness/agent.go`, `SystemPrompt` = TODO
  for all modes) are deliberately reserved for the author's own future agent work.
  The kernel must leave this seam open and must **not** fill it during the rewrite.
- `internal/core/registry.go` is a placeholder reserved alongside the agent work —
  not to be confused with the new plugin **registry** described here.

## 11. Open questions

1. **`Ctx` verb set.** Is `Send`/`Chat`/`Shell`/`OpenList`/`SetLayout` the right
   *minimal* set, or is a verb missing that makes a class of plugins impossible
   (e.g. "read the whole conversation", "spawn a sub-agent")? Under-powering forces a
   later breaking change.
2. **`RenderCtx` scope.** Does the `chat_output` port reveal a clean read surface, or
   does it balloon `RenderCtx` past what's reasonable to freeze as public API?
3. **Component v1 vs v2.** Ship commands + hooks + layout-override first, and defer
   third-party *components* (and thus a frozen `RenderCtx`) to v2? Or commit to
   components now?
4. **State model re-test.** Does central-store + read-views hold up for an async,
   multi-backend, streaming UI, or does a class of plugin want MVU-style messages?
5. **Distribution.** Is compiled-only acceptable long-term, or will a no-toolchain
   audience eventually force a yaegi/RPC escape hatch alongside it?
6. **Provider tiers.** Is plums' backend + runtime split enough, or is t3code's third
   tier — a configured/authed **instance** between definition and live runtime — worth
   adopting (e.g. for multiple concurrent connections to the same backend)?
```
