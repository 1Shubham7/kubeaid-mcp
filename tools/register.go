package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

// RegisterAll registers every MCP tool on the server, wiring each to the
// Kubernetes client.
func RegisterAll(server *mcp.Server, kc *k8s.Client) {
	registerListNamespaces(server, kc)
}
