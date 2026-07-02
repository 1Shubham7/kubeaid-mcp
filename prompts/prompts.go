// Package prompts implements the MCP prompts exposed by kubeaid-mcp.
//
// A prompt is a user-invoked, parameterized message template (surfaced as a
// slash command / menu item in the client). It does not touch the cluster; it
// returns messages that seed the conversation and steer the model toward using
// the server's tools in a sensible order.
package prompts

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterAll registers every prompt on the server.
func RegisterAll(server *mcp.Server) {
	registerDiagnosePod(server)
}

func registerDiagnosePod(server *mcp.Server) {
	server.AddPrompt(&mcp.Prompt{
		Name:        "diagnose_pod",
		Description: "Diagnose why a pod is unhealthy and suggest a fix, using the kubeaid tools.",
		Arguments: []*mcp.PromptArgument{
			{Name: "namespace", Description: "namespace of the pod", Required: true},
			{Name: "pod_name", Description: "name of the pod", Required: true},
			{Name: "context", Description: "kubeconfig context to target (optional)", Required: false},
		},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		args := req.Params.Arguments
		ns, pod := args["namespace"], args["pod_name"]
		if ns == "" || pod == "" {
			return nil, fmt.Errorf("namespace and pod_name are required")
		}

		contextNote := ""
		if c := args["context"]; c != "" {
			contextNote = fmt.Sprintf(" (context %q)", c)
		}

		text := fmt.Sprintf(`Diagnose why pod %q in namespace %q%s is unhealthy.

Work through it methodically:
1. Call describe_pod to inspect its phase, container states (waiting/termination reasons), and recent events.
2. If a container is crashing or restarting, call get_pod_logs with previous=true to read the crashed instance's logs.
3. Identify the root cause and explain it in plain language, then suggest a concrete fix.`,
			pod, ns, contextNote)

		return &mcp.GetPromptResult{
			Description: fmt.Sprintf("Diagnose pod %s/%s", ns, pod),
			Messages: []*mcp.PromptMessage{
				{
					Role:    "user",
					Content: &mcp.TextContent{Text: text},
				},
			},
		}, nil
	})
}
