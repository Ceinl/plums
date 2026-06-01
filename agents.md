Do not edit anything unless you were directly asked to

Produce plans in html websites for user to read is plan is robust and multistep

When changing layout, command, keybinding, or runtime config behavior, edit the
active project config files in `.agents/plums/config/` first. Update
`internal/app/defaults/` only when the built-in templates also need to change.

If this is first prompt in session, evaluate complexity of task and offer one off
    1) Stay on current branch
    2) Create new branch
    3) Create new worktree

terms:
    *you* - Agent, llm
    *me*, *us* - developer(s) of this project
    *user* - Actual user of this application

# Architecture

## Package layout

```
cmd/plums/main.go              ← 15 lines: ldflags + flags + api.Run()
internal/
├── api/
│   └── api.go                 ← wiring layer: Config, RegisterFlags, Run()
├── app/
│   ├── run.go                 ← main event loop
│   ├── cli.go                 ← CLI flag structs
│   ├── config.go              ← config path resolution & TOML parsing
│   ├── version.go             ← version metadata helpers
│   ├── keyboard.go            ← HandleKey + copyEditorSelection
│   ├── clipboard.go           ← writeClipboard (var for tests) + WriteClipboard
│   ├── session.go             ← session / model helpers
│   ├── palette.go             ← palette action handlers
│   ├── question.go            ← question reply helpers
│   ├── state.go               ← State struct + palette/skills logic
│   ├── render.go              ← RenderConfig + layout builder + default JSON
│   ├── commands.go            ← CommandConfig + default JSON
│   ├── skills.go              ← skill discovery + expansion
│   ├── defaults/
│   │   ├── defaults.go        ← embed FS for built-in configs
│   │   ├── config.toml
│   │   ├── layout.json
│   │   └── commands.json
│   └── testdata/              ← fixtures for app tests
├── core/
│   ├── registry.go            ← agent registry factory + fake outputs (stubs)
│   ├── adapter/
│   │   ├── types.go           ← domain types (Session, Model, Provider, Part, StreamEvent, ...)
│   │   └── backend.go         ← Backend interface
│   └── provider/
│       └── opencode/
│           ├── client.go      ← HTTP / SSE client implementing adapter.Backend
│           └── server.go      ← ServerProcess + startup helpers
├── ui/
│   ├── terminal.go            ← raw mode, mouse, kitty keyboard setup
│   └── tui/
│       ├── components/        ← all visual components (editor, chatlog, popup, ...)
│       ├── layout/            ← layout engine (flexbox-ish)
│       └── screen/            ← screen buffer + flush
├── keyboard/
│   └── keyboard.go            ← keyboard event parser
└── debuglog/
    └── debuglog.go            ← PLUMS_LOG debug logging
```

## Conventions

- `internal/app` contains the main logic but does NOT import provider-specific
  packages directly (except through closures passed in `Deps`).
- `internal/core/adapter` defines the domain types and `Backend` interface.
- `internal/core/provider/opencode` is the only provider right now; it imports
  `adapter` and implements `Backend`.
- `internal/api` wires everything together and is what `cmd/plums/main.go` calls.
- `cmd/plums/main.go` stays minimal: parse ldflags, call `api.RegisterFlags`,
  `flag.Parse`, then `api.Run`.
- All previously hard-coded values (timeouts, paths, clipboard command) live in
  `adapter.DefaultConfig` or `api.Config` with sensible fallbacks.
- `core/registry.go` contains temporary fake outputs so the rest of the app
  compiles. When real core logic is added, replace the stub functions.
- Default opencode server port is derived from `os.Getwd()` (the directory you
  run plums from) using an FNV hash mapped into the ephemeral range
  49152–65535. This means every project / worktree gets its own isolated
  `opencode serve` automatically, even when using the same compiled binary.
  Users can still pin a fixed port via `config.toml` if needed.
