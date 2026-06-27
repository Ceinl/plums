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

Runtime preferences that opt into `cfg.Dynamic` are stored separately in the
app-managed `~/.config/plums/state.toml`; it is not a user config format.

The bundled layout plugin ships only `zen`. The seeded `config.go` adds `split`
as a user plugin — run `plums -doctor` to see `zen (ui/layouts)` and
`split (split-layout)`.

## Development

```bash
go test ./...
go build ./...
```
