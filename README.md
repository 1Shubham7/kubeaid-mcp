# kubeaid-mcp

An MCP (Model Context Protocol) server that exposes a Kubernetes cluster to any
MCP-compatible AI client (Claude Code, Claude Desktop, Cursor, ...). The client
launches the binary as a subprocess and talks to it over stdio (JSON-RPC 2.0);
the server translates tool calls into Kubernetes API calls via `client-go`.

See [design.md](./design.md) for the architecture.

## Build

```bash
make build      # stamps the version from `git describe`
# or: go build -o kubeaid-mcp .
```

Install to `$GOBIN` (a stable path for client configs):

```bash
make install    # or: go install github.com/1shubham7/kubeaid-mcp@latest
```

## Tools

| Tool | Description |
|------|-------------|
| `list_contexts` | List the kubeconfig contexts (clusters) the server can target. |
| `list_namespaces` | List namespaces in a cluster, with status. |
| `list_pods` | List pods in a namespace (or all namespaces), with derived status, ready count, restarts and age. |
| `describe_pod` | Pod status, per-container state (waiting/termination reasons, last restart), and recent events. |
| `get_pod_logs` | Tail a container's logs; `previous: true` reads a crashed instance's prior logs. |
| `list_deployments` | List deployments with ready/up-to-date/available replica counts and age. |
| `list_nodes` | List cluster nodes with Ready status, roles, version and internal IP. |
| `get_events` | Recent events in a namespace (or all), sorted oldest to newest. |
| `describe_resource` | Fetch any resource kind (incl. CRDs) by kind/plural/short-name and name, via the dynamic client. |

Every tool accepts an optional `context` argument to target a specific
kubeconfig context. Omit it to use the server's default context.

## Flags

| Flag | Default | Meaning |
|------|---------|---------|
| `--kubeconfig` | `$KUBECONFIG` or `~/.kube/config` | Path to the kubeconfig file. |
| `--context` | kubeconfig current-context | Default context; individual tool calls can override it. |
| `--request-timeout` | `30s` | Per-request timeout for Kubernetes API calls. |

## Connect to Claude Code

```bash
claude mcp add kubeaid -- /absolute/path/to/kubeaid-mcp --context kind-kubeaid
```

Restart the Claude Code session to pick up new tools or a rebuilt binary.

## Connect to Claude Desktop

Recent Claude Desktop builds gate local MCP servers behind a setting, so the
order matters:

1. **Enable local MCP:** Settings → Developer → **Local MCP servers**. Local
   stdio servers are disabled by default; opening/enabling this is required.
2. **Edit the config:** on that same page click **Edit Config**. It opens the
   file the app actually reads — `~/.config/Claude/claude_desktop_config.json`
   on Linux (macOS: `~/Library/Application Support/Claude/`, Windows:
   `%APPDATA%\Claude\`). Add a top-level `mcpServers` key:

   ```json
   {
     "mcpServers": {
       "kubeaid": {
         "command": "/absolute/path/to/kubeaid-mcp",
         "args": ["--context", "kind-kubeaid"]
       }
     }
   }
   ```

   If the file already has other keys (e.g. `preferences`), keep them and add
   `mcpServers` alongside — don't overwrite the file.
3. **Fully quit and reopen** Claude Desktop. Closing the window is not enough on
   Linux — the process must actually exit. The server then appears under
   Settings → Developer → Local MCP servers.

Notes:
- The `command` must be an absolute path; GUI apps don't inherit your shell
  `PATH`.
- If your account is enterprise-managed, an admin policy can disable local MCP
  entirely, in which case no local config will load.

## Development

Drive the server by hand (no AI client needed) to inspect the raw protocol:

```bash
python3 scripts/drive.py   # sends initialize + tools/list + tools/call
```
