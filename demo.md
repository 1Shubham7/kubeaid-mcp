# kubeaid-mcp - 2-minute demo

## The pitch (say this at the top, ~15s)

> "kubeaid-mcp is an MCP server that plugs a Kubernetes cluster straight into
> Claude. No new UI, no kubectl - I just talk to my clusters. It's read-only by
> default, writes are opt-in and gated, and one server handles every cluster in
> my kubeconfig. Let me show you two things: fixing a broken workload, and a
> full deploy-then-decommission - across two clusters."

---

## Before you record (one-time setup - NOT part of the 2 minutes)

### 1. Create two kind clusters

```bash
kind create cluster --name dev     # context: kind-dev
kind create cluster --name prod    # context: kind-prod  (our stand-in "prod")
```

### 2. Pre-pull images into both clusters

So nothing stalls on an image pull mid-demo:

```bash
docker pull postgres:16
docker pull nginx:1.27-alpine
for c in dev prod; do
  kind load docker-image postgres:16 --name "$c"
  kind load docker-image nginx:1.27-alpine --name "$c"
done
```

### 3. Wire kubeaid into Claude Desktop with writes enabled

Build/refresh the binary:

```bash
make install    # installs to ~/go/bin/kubeaid-mcp
```

Then make sure Claude Desktop's config
(`~/.config/Claude/claude_desktop_config.json`) has kubeaid registered with
`--allow-writes` and **no** `--context` (so it follows your kubeconfig live):

```json
{
  "mcpServers": {
    "kubeaid": {
      "command": "/home/shubham/go/bin/kubeaid-mcp",
      "args": ["--allow-writes"]
    }
  }
}
```

Keep any existing `preferences` / `coworkUserFilesPath` keys in that file - add
`mcpServers` alongside them, don't overwrite. Then enable Settings → Developer →
**Local MCP servers**. (Full details in
[README](./README.md#3-register-with-claude-desktop).) No `--context` means the
server follows your kubeconfig's current-context live - exactly how we switch
clusters mid-demo.

### 4. Pre-stage the break (so we have something real to fix)

Deploy a Postgres that will **CrashLoopBackOff** on purpose - its required
`POSTGRES_PASSWORD` is missing. This is a real, common misconfiguration, not a
toy.

```bash
kubectl --context kind-dev apply -f - <<'EOF'
apiVersion: v1
kind: Namespace
metadata:
  name: payments
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
  namespace: payments
spec:
  replicas: 1
  selector:
    matchLabels: { app: postgres }
  template:
    metadata:
      labels: { app: postgres }
    spec:
      containers:
        - name: postgres
          image: postgres:16
          ports:
            - containerPort: 5432
          # BUG (intentional): POSTGRES_PASSWORD is required and omitted,
          # so the container exits on startup -> CrashLoopBackOff.
          resources:
            requests: { memory: "128Mi", cpu: "100m" }
            limits:   { memory: "256Mi", cpu: "500m" }
EOF
```

Give it ~30s, then confirm it's actually crashing before you hit record:

```bash
kubectl --context kind-dev -n payments get pods
# STATUS should read CrashLoopBackOff / Error
```

### 5. Start on the dev cluster

```bash
kubectl config use-context kind-dev
```

**Fully quit and reopen Claude Desktop** (closing the window isn't enough on
Linux) so it relaunches the server with `--allow-writes`. Confirm kubeaid shows
up under Settings → Developer → Local MCP servers. You're ready to record.

---

## The 2-minute run

Times are a guide; the whole thing is four prompts and one context switch.

### 0:00 - 0:20 · Frame it

Say the pitch above. Then, on camera, show the two clusters exist - one quick
prompt:

> **Type:** `list all the contexts you can reach, and show me the nodes of the current one`

Claude lists both clusters it can reach and the nodes of the current one -
multi-cluster reach, shown in one shot.

### 0:20 - 1:00 · Task 1 (kind-dev): find and fix the crashing database

This is the money shot - detect → diagnose from logs → fix → verify, all from
chat.

> **Type:** `Something is wrong in the payments namespace - a pod won't stay up. Find out why and fix it.`

What the audience watches Claude do, on its own:
- finds `postgres` stuck in CrashLoopBackOff
- reads the container's crash logs and surfaces the real error:
  *"Database is uninitialized and superuser password is not specified..."*
- explains the root cause in plain English
- patches the live deployment to add the missing `POSTGRES_PASSWORD`
- Claude Desktop asks you to confirm before it changes anything - **click Allow**
  on camera; that's your safety story, live

Then verify:

> **Type:** `confirm it recovered`

Claude re-checks and `postgres` is now **Running / 1/1 Ready**. Broken DB fixed
without a single kubectl command.

> Say: *"It read the crash logs, told me the cause, and patched the live
> deployment - and asked my permission before it changed anything."*

### 1:00 - 1:10 · Switch clusters (live)

Cut to a terminal and switch context - the server follows it, no
reconfiguration:

```bash
kubectl config use-context kind-prod
```

> **Type:** `which cluster are you on now?`

Claude reports `kind-prod` as the default context. Point out: *"Same chat, same
server - it just followed my kubeconfig to the other cluster."*

### 1:10 - 1:50 · Task 2 (kind-prod): provision, then decommission

Full lifecycle on the second cluster.

> **Type:** `Deploy a web app on this cluster: namespace "storefront", an nginx deployment called web with 2 replicas, and a Service in front of it. Then show me it's healthy.`

Claude writes the manifests and applies them (one Allow click), then checks the
rollout → **2/2 ready**, Service created. A real app, stood up from one sentence.

Now tear it down cleanly:

> **Type:** `We're decommissioning storefront. Delete the whole namespace and confirm it's gone.`

Claude deletes the whole namespace (Allow click - a **destructive** action, so
the confirmation is loud), then verifies `storefront` is gone.

> Say: *"Deploy and cleanup, on a different cluster, in two sentences."*

### 1:50 - 2:00 · Close

> "That's real production-shaped work - triage a failing database, provision an
> app, decommission it - across two clusters, from plain English. Read-only by
> default, every write gated behind a flag and a confirmation, and production
> contexts can be marked off-limits entirely. That's kubeaid-mcp."

---

## Reset between takes

```bash
kubectl --context kind-dev delete namespace payments --ignore-not-found
kubectl --context kind-prod delete namespace storefront --ignore-not-found
# then re-run setup step 4 to re-stage the break
```

## Full teardown

```bash
kind delete cluster --name dev
kind delete cluster --name prod
```

---

## Talk-track cheat sheet (the four prompts)

1. `list all the contexts you can reach, and show me the nodes of the current one`
2. `Something is wrong in the payments namespace - a pod won't stay up. Find out why and fix it.`  → then `confirm it recovered`
3. *(switch context in terminal)* → `which cluster are you on now?`
4. `Deploy a web app on this cluster: namespace "storefront", an nginx deployment called web with 2 replicas, and a Service in front of it. Then show me it's healthy.`  → then `We're decommissioning storefront. Delete the whole namespace and confirm it's gone.`

## Tips for a clean recording

- Do the image pulls and the break (setup 2 & 4) **before** recording so nothing
  waits on a pull or a CrashLoop timer.
- Keep the Claude Desktop confirm dialogs **in frame** - they *are* the safety
  story; don't hide them.
- If a step is slow, you pre-verified state in setup, so you can narrate over it.
- For an even slicker take, swap prompt #2 for the shipped `/diagnose_pod`
  slash command (`namespace: payments`, `pod_name:` the crashing pod) - same
  investigation, kicked off in one click.
