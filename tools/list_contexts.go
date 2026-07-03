package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

type listContextsInput struct{}

type listContextsOutput struct {
	Contexts []k8s.ContextInfo `json:"contexts"`
}

func registerListContexts(server *mcp.Server, kc *k8s.ClientManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_contexts",
		Description: "List the kubeconfig contexts (clusters) this server can target. The one marked isDefault is used when a tool call omits the context parameter.",
		Annotations: readOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ listContextsInput) (*mcp.CallToolResult, listContextsOutput, error) {
		contexts, err := kc.Contexts()
		if err != nil {
			return nil, listContextsOutput{}, err
		}
		return nil, listContextsOutput{Contexts: contexts}, nil
	})
}
