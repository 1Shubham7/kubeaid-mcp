# kubeaid-mcp

An MCP (Model Context Protocol) server that exposes a Kubernetes cluster to any
MCP-compatible AI client (Claude Code, Claude Desktop, Cursor, ...). The client
launches the binary as a subprocess and talks to it over stdio (JSON-RPC 2.0);
the server translates tool calls into Kubernetes API calls via `client-go`.

See [design.md](./design.md) for the architecture.

## Demo

[![kubeaid-mcp demo](https://img.youtube.com/vi/QFLlo8RDWf8/maxresdefault.jpg)](https://www.youtube.com/watch?v=QFLlo8RDWf8)

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

## Prompts

Prompts are user-invoked workflow templates (surfaced as slash commands / menu
items in the client). Unlike tools, a prompt does not touch the cluster — it
returns a message that guides the model through a task using the tools above.

| Prompt | Arguments | What it does |
|--------|-----------|--------------|
| `diagnose_pod` | `namespace`, `pod_name`, `context?` | Walk through describe → logs to find why a pod is unhealthy and suggest a fix. |
| `triage_namespace` | `namespace`, `context?` | Find every unhealthy workload in a namespace and investigate each. |
| `cluster_health_check` | `context?` | High-level health sweep: nodes, pods across all namespaces, recent warnings. |
| `review_warnings` | `namespace?`, `context?` | Review recent Warning events and explain what they mean. |

## Installation

Install the binary once, then register it with your AI client(s).

### 1. Install the binary

Pick whichever you prefer:

**Go install** (requires Go 1.26+):

```bash
go install github.com/1shubham7/kubeaid-mcp@latest
```

Installs to `$(go env GOPATH)/bin` (usually `~/go/bin`) - make sure that's on
your `PATH`.

**Prebuilt release binary:** download the archive for your OS/arch from the
[Releases](https://github.com/1shubham7/kubeaid-mcp/releases) page, then move it
onto your `PATH`:

```bash
tar xzf kubeaid-mcp-*-linux-amd64.tar.gz
sudo mv kubeaid-mcp /usr/local/bin/
```

**From source:**

```bash
git clone https://github.com/1shubham7/kubeaid-mcp.git
cd kubeaid-mcp
make install        # installs to $GOBIN / ~/go/bin (stamps the version)
# or: make build    # just builds ./kubeaid-mcp in the repo
```

Then note the absolute path - the AI clients need it:

```bash
command -v kubeaid-mcp
```

### 2. Register with Claude Code

```bash
claude mcp add kubeaid -- "$(command -v kubeaid-mcp)"
```

With no `--context`, the server follows your kubeconfig's current-context live,
so `kubectl config use-context <name>` switches the target cluster without
re-registering. Pin a fixed default with `--context kind-kubeaid` if you'd
rather it never move.

Read tools only are exposed by default. To enable the mutating tools
(apply/patch/delete/scale/rollout), add `--allow-writes`, and list any
production contexts as protected so they can never be written to:

```bash
claude mcp add kubeaid -- "$(command -v kubeaid-mcp)" \
  --allow-writes \
  --protected-context prod-cluster,another-prod-cluster
```

### 3. Register with Claude Desktop

Recent Claude Desktop builds gate local MCP servers behind a setting, so order
matters:

1. **Enable local MCP:** Settings → Developer → **Local MCP servers**. Local
   stdio servers are off by default; opening/enabling this is required.
2. **Edit the config:** on that page click **Edit Config** - it opens the file
   the app actually reads (`~/.config/Claude/claude_desktop_config.json` on
   Linux; macOS `~/Library/Application Support/Claude/`; Windows
   `%APPDATA%\Claude\`). Add a top-level `mcpServers` key, using the absolute
   path from step 1:

   ```json
   {
     "mcpServers": {
       "kubeaid": {
         "command": "/home/you/go/bin/kubeaid-mcp"
       }
     }
   }
   ```

   If the file already has other keys (e.g. `preferences`), keep them and add
   `mcpServers` alongside - don't overwrite the file.

3. **Fully quit and reopen** Claude Desktop. Closing the window is not enough on
   Linux - the process must actually exit. The server then appears under
   Settings → Developer → Local MCP servers.

**Enabling writes:** add the flags to `args` - e.g. make the local cluster
writable while protecting production:

```json
{
  "mcpServers": {
    "kubeaid": {
      "command": "/home/you/go/bin/kubeaid-mcp",
      "args": [
        "--context", "kind-kubeaid",
        "--allow-writes",
        "--protected-context", "prod-cluster,another-prod-cluster"
      ]
    }
  }
}
```

Add `--allow-exec` for the `exec_command` tool, and re-run step 3 after changing
`args`.

**Notes:**

- `command` must be an absolute path; GUI apps don't inherit your shell `PATH`.
- Destructive tools carry a `DestructiveHint`, so Desktop still prompts you per
  action - the flags control what's *possible*; the prompt is your confirmation.
- If your account is enterprise-managed, an admin policy can disable local MCP
  entirely, in which case no local config will load.

## Safety

The server is **read-only by default** - the read tools only call non-mutating
verbs (get, list, watch, log). Mutating tools exist but are gated:

- `--allow-writes` must be set for `apply_manifest`, `patch_resource`,
  `delete_resource`, `scale_deployment`, and `rollout_restart` to be exposed at
  all.
- `--allow-exec` (in addition) is required for `exec_command`.
- `--protected-context` lists contexts that may **never** be written to or
  exec'd into, even with the flags above - put your production contexts here.
- Tools are annotated (`ReadOnlyHint` / `DestructiveHint`) so clients can prompt
  before risky actions.

The server authenticates with your kubeconfig credentials, so it can only do
what your account is already permitted to do.

## Flags

| Flag | Default | Meaning |
|------|---------|---------|
| `--kubeconfig` | `$KUBECONFIG` or `~/.kube/config` | Path to the kubeconfig file. |
| `--context` | follows kubeconfig current-context (live) | Pin a fixed default context. Omit it to track current-context live, so `kubectl config use-context` switches the cluster mid-session. Individual tool calls can always override it. |
| `--request-timeout` | `30s` | Per-request timeout for Kubernetes API calls. |
| `--allow-writes` | `false` | Expose the mutating tools (apply/patch/delete/scale/rollout). |
| `--allow-exec` | `false` | Expose `exec_command` (run commands in containers). |
| `--protected-context` | none | Comma-separated contexts that may never be written to or exec'd into. |

## Development

Drive the server by hand (no AI client needed) to inspect the raw protocol:

```bash
python3 scripts/drive.py   # sends initialize + tools/list + tools/call
```
