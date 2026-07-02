# kubeaid-mcp

An MCP (Model Context Protocol) server that exposes a Kubernetes cluster to any
MCP-compatible AI client (Claude Code, Claude Desktop, Cursor, ...). The client
launches the binary as a subprocess and talks to it over stdio (JSON-RPC 2.0);
the server translates tool calls into Kubernetes API calls via `client-go`.

See [design.md](./design.md) for the architecture.

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

### Write tools (opt-in)

These are registered only when `--allow-writes` is set. Each accepts an optional
`dry_run` to simulate the change server-side without persisting it.

| Tool | Description |
|------|-------------|
| `apply_manifest` | Create or update resources from a YAML/JSON manifest (server-side apply). |
| `patch_resource` | Patch an existing resource (strategic / merge / json). |
| `delete_resource` | Delete a resource by kind and name. |
| `scale_deployment` | Set a deployment's replica count. |
| `rollout_restart` | Rolling-restart a deployment/statefulset/daemonset. |
| `exec_command` | Run a command inside a container (only with `--allow-exec`). |

## Safety

The server is **read-only by default** — the read tools only call non-mutating
verbs (get, list, watch, log). Mutating tools exist but are gated:

- `--allow-writes` must be set for `apply_manifest`, `patch_resource`,
  `delete_resource`, `scale_deployment`, and `rollout_restart` to be exposed at
  all.
- `--allow-exec` (in addition) is required for `exec_command`.
- `--protected-context` lists contexts that may **never** be written to or
  exec'd into, even with the flags above — put your production contexts here.
- Tools are annotated (`ReadOnlyHint` / `DestructiveHint`) so clients can prompt
  before risky actions.

The server authenticates with your kubeconfig credentials, so it can only do
what your account is already permitted to do.

## Flags

| Flag | Default | Meaning |
|------|---------|---------|
| `--kubeconfig` | `$KUBECONFIG` or `~/.kube/config` | Path to the kubeconfig file. |
| `--context` | kubeconfig current-context | Default context; individual tool calls can override it. |
| `--request-timeout` | `30s` | Per-request timeout for Kubernetes API calls. |
| `--allow-writes` | `false` | Expose the mutating tools (apply/patch/delete/scale/rollout). |
| `--allow-exec` | `false` | Expose `exec_command` (run commands in containers). |
| `--protected-context` | none | Comma-separated contexts that may never be written to or exec'd into. |

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

### Enabling writes (and protecting production)

The write tools are opt-in. To enable them, add the flags to `args`. The
example below makes the local `kind-kubeaid` cluster writable while marking a
production context as protected, so the AI can never mutate or exec into it —
even though writes are enabled globally:

```json
{
  "mcpServers": {
    "kubeaid": {
      "command": "/absolute/path/to/kubeaid-mcp",
      "args": [
        "--context", "kind-kubeaid",
        "--allow-writes",
        "--protected-context", "prod-cluster,another-prod-cluster"
      ]
    }
  }
}
```

- Add `--allow-exec` to `args` as well if you want the `exec_command` tool.
- `--protected-context` takes a comma-separated list; those contexts reject all
  writes and exec regardless of `--allow-writes` / `--allow-exec`.
- Because destructive tools carry a `DestructiveHint`, Claude Desktop still
  prompts you to approve each risky action at call time — the flags control what
  is *possible*; the prompt is your per-action confirmation.
- Re-run step 3 (fully quit and reopen) after changing `args`.

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

## Build

```bash
make build      # stamps the version from `git describe`
# or: go build -o kubeaid-mcp .
```

Install to `$GOBIN` (a stable path for client configs):

```bash
make install    # or: go install github.com/1shubham7/kubeaid-mcp@latest
```

## Releasing

CI runs vet + build + tests on every push and PR to `main`. Pushing a `v*` tag
triggers a GoReleaser run that builds cross-platform binaries (linux/darwin/
windows, amd64/arm64) and publishes a GitHub Release with a changelog:

```bash
git tag v0.1.0
git push origin v0.1.0
```

No extra secrets are needed — the release workflow uses the built-in
`GITHUB_TOKEN`.
