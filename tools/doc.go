// Package tools implements the MCP tools exposed by kubeaid-mcp.
//
// Every tool in this package is READ-ONLY: handlers may only call
// non-mutating Kubernetes verbs (get, list, watch, and log streaming). No tool
// may create, update, patch, or delete a resource. Any future write capability
// must be a separate, explicitly opt-in surface, not added to these tools.
package tools
