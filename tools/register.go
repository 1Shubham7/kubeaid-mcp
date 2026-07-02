package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

// RegisterAll registers every MCP tool on the server, wiring each to the
// Kubernetes client manager.
func RegisterAll(server *mcp.Server, kc *k8s.ClientManager) {
	registerListContexts(server, kc)
	registerListNamespaces(server, kc)
	registerListPods(server, kc)
	registerDescribePod(server, kc)
	registerGetPodLogs(server, kc)
	registerListDeployments(server, kc)
	registerListNodes(server, kc)
	registerGetEvents(server, kc)
	registerDescribeResource(server, kc)
}
