<div align="center">

<h1> Plums </h1>

</div>

## Configuration

Plums reads local configuration from `.agents/plums/config/config.toml`.

To connect Plums to an opencode server at a specific URL, set `opencode_server_url` under `[opencode]`:

```toml
[opencode]
opencode_server_url = "http://127.0.0.1:4096"
```

If `opencode.opencode_server_url` is omitted, Plums uses `http://127.0.0.1:4096`.
