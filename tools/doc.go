// Package tools implements the MCP tools exposed by kubeaid-mcp.
//
// Tools are split into two groups:
//
//   - Read-only tools (get, list, watch, log) are always registered.
//   - Mutating tools (apply, patch, delete, scale, rollout restart) and exec are
//     registered only when the server is started with --allow-writes /
//     --allow-exec, and every mutation is additionally refused on any context
//     named in --protected-context.
//
// Read-only tools must never call a mutating verb. Mutating tools must call
// ClientManager.EnsureWritable (or EnsureExecAllowed) before touching the
// cluster, so the write gate cannot be bypassed.
package tools
