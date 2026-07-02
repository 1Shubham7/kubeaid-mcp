package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

// RegisterAll registers the MCP tools on the server. Read-only tools are always
// registered; mutating tools only when writes are enabled, and exec only when
// exec is enabled.
func RegisterAll(server *mcp.Server, kc *k8s.ClientManager) {
	// Read-only tools.
	registerListContexts(server, kc)
	registerListNamespaces(server, kc)
	registerListPods(server, kc)
	registerDescribePod(server, kc)
	registerGetPodLogs(server, kc)
	registerListDeployments(server, kc)
	registerListNodes(server, kc)
	registerGetEvents(server, kc)
	registerDescribeResource(server, kc)

	// Mutating tools (opt-in via --allow-writes).
	if kc.WritesEnabled() {
		registerApplyManifest(server, kc)
		registerPatchResource(server, kc)
		registerDeleteResource(server, kc)
		registerScaleDeployment(server, kc)
		registerRolloutRestart(server, kc)
	}

	// Exec (opt-in via --allow-exec).
	if kc.ExecEnabled() {
		registerExecCommand(server, kc)
	}
}
