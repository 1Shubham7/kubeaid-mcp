# Kubeaid MCP Server - High Level Design

## Overview

A local Go binary that sits between any MCP-compatible AI client (Claude Desktop,
Claude Code, Cursor, etc.) and a Kubernetes cluster. The AI talks to the MCP server
over stdio using the MCP protocol. The MCP server talks to the Kubernetes API server
using `client-go`. The user never writes a `kubectl` command.

---

## 1. System Context

Where the MCP server sits relative to everything else.

```mermaid
C4Context
    Person(user, "User", "SRE / Developer asking questions in natural language")

    System(ai_client, "AI Client", "Claude Desktop, Claude Code, Cursor, or any MCP-compatible app")

    System(mcp_server, "kubeaid-mcp", "Go binary on the user's laptop. Translates MCP tool calls into Kubernetes API calls.")

    System_Ext(k8s, "Kubernetes Cluster", "Any cluster the user's kubeconfig points at — Hetzner, GKE, kind, etc.")

    Rel(user, ai_client, "Asks questions in natural language")
    Rel(ai_client, mcp_server, "MCP protocol over stdio (JSON-RPC 2.0)")
    Rel(mcp_server, k8s, "HTTPS via Kubernetes API (client-go)")
```

---

## 2. End-to-End Request Flow

What happens from the moment a user types a question to when they get an answer.

```mermaid
sequenceDiagram
    actor User
    participant Claude as AI Client<br/>(Claude Desktop)
    participant MCP as kubeaid-mcp<br/>(Go binary / stdio)
    participant K8s as kube-apiserver<br/>(HTTPS :6443)

    User->>Claude: "Why is my nginx pod crashing in staging?"

    Note over Claude: Model decides which tools to call

    Claude->>MCP: tools/call → list_pods(namespace="staging")
    MCP->>K8s: GET /api/v1/namespaces/staging/pods
    K8s-->>MCP: JSON pod list
    MCP-->>Claude: Pod "nginx-7d9f-xkq2p" status=CrashLoopBackOff, restarts=14

    Claude->>MCP: tools/call → describe_pod(namespace="staging", pod="nginx-7d9f-xkq2p")
    MCP->>K8s: GET /api/v1/namespaces/staging/pods/nginx-7d9f-xkq2p
    K8s-->>MCP: Full pod spec + events
    MCP-->>Claude: Events: "Back-off restarting failed container"

    Claude->>MCP: tools/call → get_pod_logs(namespace="staging", pod="nginx-7d9f-xkq2p", lines=50)
    MCP->>K8s: GET /api/v1/namespaces/staging/pods/nginx-7d9f-xkq2p/log?tailLines=50
    K8s-->>MCP: Log tail
    MCP-->>Claude: "panic: cannot connect to postgres: connection refused"

    Claude-->>User: "Your nginx pod is crash-looping because it can't reach Postgres.\nCheck if the postgres service is up in staging and verify DB_HOST env var."
```

---

## 3. MCP Protocol Handshake

What happens at the transport level when the AI client first spawns your binary.

```mermaid
sequenceDiagram
    participant Client as AI Client
    participant Server as kubeaid-mcp

    Note over Client: Reads claude_desktop_config.json,<br/>spawns the binary as a subprocess

    Client->>Server: initialize {protocolVersion, capabilities}
    Server-->>Client: {serverInfo: {name: "kubeaid-mcp"}, capabilities: {tools: {}}}

    Client->>Server: notifications/initialized
    Note over Client,Server: Handshake complete — normal operation begins

    Client->>Server: tools/list
    Server-->>Client: [{name: "list_pods"}, {name: "describe_pod"}, ...]

    Note over Client: Model now knows what tools exist.<br/>Will call them when relevant.
```

---

## 4. Internal Architecture of the MCP Server

How the Go binary is structured internally.

```mermaid
flowchart TD
    subgraph binary ["kubeaid-mcp binary"]
        main["main.go\n─────────\nParse flags\nBuild kubeconfig\nRegister tools\nserver.Run()"]

        subgraph tools ["tools/ package"]
            t0["list_contexts.go"]
            t1["list_namespaces.go"]
            t2["list_pods.go"]
            t3["describe_pod.go"]
            t4["get_pod_logs.go"]
            t5["list_deployments.go"]
            t6["list_nodes.go"]
            t7["get_events.go"]
            t8["describe_resource.go"]
        end

        subgraph k8sclient ["k8s/ package"]
            kc["client.go — ClientManager\n─────────\nNewClientManager()\nClientset(context)\nDynamicClient(context)\nper-context client cache"]
        end

        sdk["go-sdk\n(github.com/modelcontextprotocol/go-sdk)\n─────────\nStdioTransport\nmcp.AddTool()\nJSON-RPC framing"]
    end

    stdio["stdin / stdout\n(JSON-RPC 2.0)"]
    kubeapi["kube-apiserver\n(HTTPS)"]

    stdio <-->|MCP protocol| sdk
    sdk --> main
    main --> tools
    main --> k8sclient
    tools --> k8sclient
    k8sclient <-->|client-go| kubeapi
```

---

## 5. Multi-Cluster Configuration

A single server process serves every cluster in the kubeconfig. Rather than
launching one process per cluster, the server exposes a `list_contexts` tool and
gives every other tool an optional `context` parameter, so the AI picks the
cluster at call time. The `--context` flag only sets the *default* used when a
call omits one; when the flag is omitted, the default tracks the kubeconfig's
current-context live (re-read from disk each call), so `kubectl config
use-context` switches the target mid-session without a restart. Clients are
built and cached per context on first use, so switching clusters
mid-conversation needs no restart.

```mermaid
flowchart LR
    subgraph config ["~/.kube/config"]
        ctx1["context: prod-hetzner"]
        ctx2["context: staging-hetzner"]
        ctx3["context: kind-local"]
    end

    subgraph claude_config ["client config (one entry)"]
        s1["kubeaid\n→ kubeaid-mcp --context kind-local"]
    end

    subgraph process ["1 server process (context-aware)"]
        p1["kubeaid-mcp"]
        cache["per-context client cache\nprod → clientset+dynamic\nstaging → clientset+dynamic\nlocal → clientset+dynamic"]
    end

    subgraph clusters ["Kubernetes Clusters"]
        c1["Hetzner Prod"]
        c2["Hetzner Staging"]
        c3["kind (local)"]
    end

    s1 -->|spawns| p1
    p1 --> cache
    cache -->|reads| ctx1
    cache -->|reads| ctx2
    cache -->|reads| ctx3

    cache --> c1
    cache --> c2
    cache --> c3
```

---

## 6. Tool Inventory

Every tool the server exposes and what it calls. All tools additionally accept
an optional `context` parameter to target a specific kubeconfig context; the
`namespace` parameter is optional on the list tools (omit to span all
namespaces).

```mermaid
flowchart LR
    subgraph tools ["MCP Tools (read-only)"]
        t0["list_contexts\nparams: none"]
        t1["list_namespaces\nparams: none"]
        t2["list_pods\nparams: namespace?"]
        t3["describe_pod\nparams: namespace, pod_name"]
        t4["get_pod_logs\nparams: namespace, pod_name, lines?, container?, previous?"]
        t5["list_deployments\nparams: namespace?"]
        t6["list_nodes\nparams: none"]
        t7["get_events\nparams: namespace?"]
        t8["describe_resource\nparams: kind, name, namespace?, api_version?"]
    end

    subgraph k8s_api ["Kubernetes API Endpoints"]
        e0["reads kubeconfig (no API call)"]
        e1["GET /api/v1/namespaces"]
        e2["GET /api/v1/namespaces/{ns}/pods"]
        e3["GET /api/v1/namespaces/{ns}/pods/{name}"]
        e4["GET /api/v1/namespaces/{ns}/pods/{name}/log"]
        e5["GET /apis/apps/v1/namespaces/{ns}/deployments"]
        e6["GET /api/v1/nodes"]
        e7["GET /api/v1/namespaces/{ns}/events"]
        e8["dynamic client + discovery\nresolves any kind → GET"]
    end

    t0 --> e0
    t1 --> e1
    t2 --> e2
    t3 --> e3
    t4 --> e4
    t5 --> e5
    t6 --> e6
    t7 --> e7
    t8 --> e8
```

### Write tools (opt-in)

The tools above are read-only and always registered. Mutating tools are
registered only when the server is started with `--allow-writes`, and exec only
with `--allow-exec`. Every mutation is refused on any context named in
`--protected-context`, and each accepts `dry_run` to simulate server-side.
Tools carry MCP annotations (`ReadOnlyHint` / `DestructiveHint`) so clients can
prompt before risky actions.

| Tool | Verb | Client |
|------|------|--------|
| `apply_manifest` | server-side apply (create/update) | dynamic |
| `patch_resource` | patch (strategic/merge/json) | dynamic |
| `delete_resource` | delete | dynamic |
| `scale_deployment` | update scale subresource | typed |
| `rollout_restart` | patch pod-template annotation | dynamic |
| `exec_command` | pod exec (SPDY stream) | rest.Config |

---

## 7. Kubeconfig Auth Flow

How the server authenticates to the Kubernetes API. It uses the kubeconfig on
disk (the same credentials `kubectl` uses); in-cluster service-account auth is a
possible future addition, not yet implemented.

```mermaid
flowchart TD
    start["server starts\nload kubeconfig"]

    start -->|"clientcmd loading rules\n(--kubeconfig, else KUBECONFIG, else ~/.kube/config)"| local["ClientConfig with context override\n─────────────────\nReads kubeconfig file\nApplies per-call context (or default)\nSets request timeout\nLoads: API server URL, client cert/key or token"]

    local --> clientset["kubernetes.NewForConfig(config)\ndynamic.NewForConfig(config)\n→ typed + dynamic clients, cached per context"]

    clientset --> calls["Tool handlers call:\nclientset.CoreV1().Pods(ns).List()\nclientset.AppsV1().Deployments(ns).List()\ndynamic client for describe_resource\netc."]
```

---

## 8. Deployment & Installation

How a user gets this running in under 2 minutes.

```mermaid
flowchart LR
    A["go install\ngithub.com/1shubham7/kubeaid-mcp@latest"]
    B["Register the server once\n(Claude Code: claude mcp add;\nClaude Desktop: enable local MCP,\nEdit Config, add one entry)"]
    C["Restart the client"]
    D["Ask Claude:\n'list my pods in staging'"]

    A --> B --> C --> D
```
