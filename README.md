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

plums is global-only, neovim style. Seed the default config:

```bash
plums -init-config
```

Config files are written to `~/.config/plums/config`:

- `config.go` — the compiled Go config (`plums.Use`); edit this to reshape plums.
  Launching `plums` auto-compiles it (cached) and runs the result; `plums build`
  builds it explicitly.
- `config.toml`, `layout.json`, `commands.json` — runtime data defaults for a stock binary.

`split` ships as a user plugin in `config.go` (not a builtin) — run `plums -doctor`
to see it registered as `split (split-layout)`.

## Development

```bash
go test ./...
go build ./...
```
