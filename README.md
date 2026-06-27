# Plums

Open source AI agent TUI.

Very early WIP.

## Install

```bash
go install github.com/Ceinl/plums/cmd/plums@latest
```

## Prerequisites

- Go 1.26.2+
- `opencode` for the default backend
- `codex` only when using the Codex backend

## Run

```bash
plums
```

Useful flags:

- `-version`
- `-provider opencode|codex`
- `-server-url URL`
- `-init-config`
- `-no-config`
- `-doctor`

## Config

plums is global-only, neovim style. On first launch, plums seeds a compiled Go
config at `~/.config/plums/config/config.go` when one is missing. You can also
seed it explicitly:

```bash
plums -init-config
```

The user-authored config directory is `~/.config/plums/config`:

- `config.go` — the compiled Go config (`config.Use`); edit this to reshape plums.
  Launching `plums` auto-compiles it (cached) and runs the result; `plums build`
  builds it explicitly.
- `go.mod` / `go.sum` — the managed config module. plums creates these; normal Go
  tooling works inside this directory.
- `plugins/` — local user plugins scaffolded by `plums plugin new`.

Runtime preferences that opt into `cfg.Dynamic` are stored separately in the
app-managed `~/.config/plums/state.toml`; it is not a user config format.

The bundled layout plugin ships only `zen`. The seeded `config.go` adds `split`
as a user plugin — run `plums -doctor` to see `zen (ui/layouts)` and
`split (split-layout)`.

## Plugins

Create a local plugin in the config module:

```bash
plums plugin new hello
```

This creates `~/.config/plums/config/plugins/hello`, adds a `plugintest` smoke
test, and wires `hello.New(hello.Options{})` into `config.go`. Use
`--no-wire` if you want to edit `config.go` manually.

Add a remote plugin module:

```bash
plums plugin add github.com/example/plums-thing@latest
```

This runs `go get` and `go mod tidy` in the config module, then wires the
package into `config.go` using the same `New(Options{})` convention. If a plugin
uses a different constructor shape, rerun with `--no-wire` and edit `config.go`.

Other plugin commands:

```bash
plums plugin list
plums plugin update
plums plugin update github.com/example/plums-thing
```

## Development

```bash
go test ./...
go build ./...
```
