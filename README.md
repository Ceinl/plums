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
- `-init-config-local`
- `-config-global` / `-cg`
- `-config-local` / `-cl`

## Config

Create local config:

```bash
plums -init-config-local
```

Config files are written to `.agents/plums/config`:

- `config.toml`
- `layout.json`
- `commands.json`

## Development

```bash
go test ./...
go build ./...
```
