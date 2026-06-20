On changing/creating a layout, command, keybinding or runtime setting, edit the embedded defaults in `internal/app/defaults` (`config.toml`, `layout.json`, `commands.json`, `config.go.tmpl`) — they are the source of truth seeded to the global config. plums is global-only (neovim style): user config lives at `~/.config/plums/config`; there is no project-local config.
On creating a new backend, all raw-protocol changes stay in the provider package under `internal/core/provider/<name>`; all domain translation stays in the bridge under `internal/core/backend/<name>`. Do not change provider-package logic unless necessary.

This project is a VERY EARLY WIP. Maintain structure, and proposing changes that improve long-term maintainability is encouraged.
Priorities:
    1. Performance first
    2. Configurability first
    3. Reliability first

terms:
    *you* - Agent, llm
    *me*, *us* - developer(s) of this project
    *user* - Actual user of this application

> **Canonical design: [`DESIGN.md`](DESIGN.md).** plums is becoming the "neovim of AI
> UIs": a small **kernel** plus a stack of **plugins**, where the built-in behavior is
> itself plugins and any of it can be replaced. Read DESIGN.md before architectural work;
> this file is the operational map.

# Architecture

## The big picture (layers, top = user-facing, bottom = irreducible)

```
cmd/plums/main.go            → plums/runtime.Main (ldflags, flags, `plums build`)
internal/api                 → wiring: resolve config → kernel.Load → app.Run
plums/                       → PUBLIC API (the plugin contract; imports nothing internal)
internal/kernel              → registry (last-wins) + plugin loader + bundled `builtin` plugins
internal/app                 → the runtime host: event loop, State store, render driver,
                               built-in components, public-component adapter
internal/core/backend/<x>    → bridge / anti-corruption layer: native types → plums types
internal/core/provider/<x>   → raw client: protocol, native types, process lifecycle
internal/ui                  → terminal raw-mode + the tui (components, layout engine, screen)
```

Dependencies point **inward**: `provider` knows only its protocol (never imports `plums`);
the `backend` bridge imports both the raw client and `plums`; `plums` imports nothing
internal. See DESIGN.md §5.4 for the ACL discipline.

## The public API — package `plums/`

This is the frozen-ish contract a plugin (or the user's compiled config) is written against.

- `plugin.go`   — `Plugin` (`Name`, `Init(Host)`), `Host`, and the optional capability
  interfaces discovered by type assertion: `CommandProvider`, `ComponentProvider`,
  `LayoutProvider`, `BackendProvider`, and the `OnMessage`/`OnSessionStart`/`OnToolCall` hooks.
- `config.go`   — `Config` (`Settings`+`Plugins`) and `Settings`. `plums.Use(cfg)` registers
  the compiled user config (the xmonad model; see DESIGN.md §8 and `plums build`).
- `component.go` — `Component` (`Name`/`Arrange`/`Render`), `Surface` (the cell drawing
  primitive), `RenderCtx` (the only read path into state), `Layout`/`Node`, and optional
  component capabilities: `ComponentInstancer` (per-slot instance), `KeyHandler`,
  `MouseHandler`, `SelectionProvider`, `Scrollable`, `DirtyTracker`.
- `command.go`  — `Command` and `Ctx` (runtime verbs: `Send`, `Chat`, `Copy`, `Shell`,
  `SetLayout`, `OpenList`, …).
- `backend.go`  — `Backend` port + `BackendRegistration` (name/label/backend/startup) + the
  domain model (`Session`, `Provider`, `Model`, `StreamEvent`, `ToolCall`, `QuestionRequest`).
- `registry.go` — `RegistryKind`/`RegistryKey` for override + `Settings.Disable`.
- `layout/`     — Go builders for layout trees (`Split`, `Column`, `Chat`, `Editor`, …).
- `runtime/`    — `runtime.Main`/`runtime.Run`: the entrypoint stock and generated binaries call.

## The kernel — `internal/kernel/`

The only non-plugin code. Stays small.

- `registry.go` — `(kind, name) → value`, **last-wins** with shadow-logging. `Disable` removes
  entries. `EntriesInRegistrationOrder` preserves load order.
- `load.go`     — `Load(cfg, opts)` activates plugins in order: **`opts.Defaults` (bundled) →
  `cfg.Plugins()` → cfg's own capabilities** (later wins; the user always wins). Discovers each
  capability by type assertion (`activateCapabilities`).
- `builtin/`    — the bundled default plugin **set** (DESIGN.md §2.1, no longer one monolith):
  `DefaultPlugins()` returns one plugin per backend (`backend/opencode`, `backend/codex`,
  `backend/claude`, `backend/claude-mirror`) plus grouped `ui/components`, `ui/layouts`,
  `ui/commands`. Each is its own registry owner, so a config can shadow or `Disable` a single one.
  `core.go` holds `DefaultCommands`/`BackendRegistrations`; `components.go`/`layouts.go` the rest.

## The runtime host — `internal/app/`

Still hosts the event loop, canonical state, and render driver (DESIGN.md §9 ports these
inward over time; today the kernel feeds them).

- `run.go`            — the main event loop: keyboard → keybinds → public components → legacy
  `HandleKey`; pumps the backend stream; dispatches hooks; drains `Ctx` mutations.
- `state.go` (+ `state_*.go`) — the **canonical store**. Components read it only via `RenderCtx`;
  mutations go through `Ctx`. Holds the per-build public-component instance cache + mouse capture.
- `public_component.go` — `publicComponentAdapter` bridges a public `plums.Component` to the
  internal `layout.Component`, builds `renderCtx`, routes key/mouse, and instantiates
  `ComponentInstancer` templates per **layout slot** (`name@slotID`).
- `component_chat_output.go` — the reference **public** component: renders only via
  `RenderCtx`+`Surface`, owns its own selection (`MouseHandler`/`SelectionProvider`) and scroll
  (`Scrollable`). No privileged `*State` path. New built-ins should follow this shape.
- `component_registry.go` / `plugin_component.go` — the *legacy* path: built-ins still wrapped as
  `pluginComponent` carrying an internal `ComponentFactory` (privileged `*State`). These are being
  ported onto the public path one at a time (Stage 4); `chat_output` is already done.
- `runtime_ctx.go` — implements `plums.Ctx`; marshals mutations back onto the event loop so
  command goroutines never touch the store directly.
- `render*.go` — builds the `layout.Component` tree from the active layout, addressing components
  by name through the registry-derived factory map.

## Backends — bridge + raw client

- `internal/core/provider/<name>` — raw client. Protocol (HTTP/SSE, JSON-RPC/stdio), native
  types in `types.go`, process lifecycle. Imports stdlib (+`debuglog`) only.
- `internal/core/backend/<name>` — the bridge. `Registration()` returns a `plums.BackendRegistration`;
  translates native↔`plums` domain types; implements optional capabilities (`ResetSession` for
  claude-mirror, `AbortSession` for opencode) as plain methods discovered by assertion.
- `internal/core/harness/` — **reserved** (agent prompts, permissions) for the author's own future
  agent work. Do not fill `SystemPrompt` during the rewrite (DESIGN.md §10).

## The TUI — `internal/ui/`

- `terminal.go` — raw mode, mouse, kitty keyboard.
- `tui/screen` — the cell buffer; `Set`/`Width`/`Height`/`Flush`. Satisfies `plums.Surface`.
- `tui/layout` — the flexbox-ish `div` engine and `layout.Component` interface.
- `tui/components` — the concrete renderers (chatlog, editor, sessions, palette, …). These draw
  through `plums.Surface` where ported (e.g. `ChatLog.RenderSurface`), with a `*screen.Screen` shim.
- `tui/theme` — the palette (`BgBase`, `Accent`, …).

## Conventions

- Add a new **backend**: new `provider/<name>` (raw) + `backend/<name>` (bridge, `Registration()`),
  then add it to `builtin.BackendRegistrations`. Keep all translation in the bridge.
- Add a new **public component**: implement `plums.Component` (+ `ComponentInstancer` if it owns
  state), register it in `builtin`/`DefaultPluginComponents`. Render only via `RenderCtx`+`Surface`;
  own ephemeral UI state on the instance (DESIGN.md §6). Mirror `component_chat_output.go`.
- Grow `RenderCtx`/`Ctx` when a built-in genuinely needs a verb — that accretion *is* the API being
  driven to completeness by dogfooding (DESIGN.md §2.4, §6). Don't add privileged `*State` reads.
- `internal/api` is the only wiring layer and is what `cmd/plums/main.go` (via `plums/runtime`) calls.
- Runtime settings come from the embedded `internal/app/defaults` TOML (or the user's global
  `~/.config/plums/config`), resolved into `plums.Settings`. The compiled-Go config
  (`plums.Use` + `plums build`) is the target path; launching plums auto-builds the user's
  `~/.config/plums/config/config.go` when present (neovim style).
- Default opencode port is derived from `os.Getwd()` via FNV hash into 49152–65535, so every
  project/worktree gets its own isolated `opencode serve`. Pin a fixed port in config if needed.
- `--doctor` prints the resolved registry with per-owner provenance; use it to verify overrides.
