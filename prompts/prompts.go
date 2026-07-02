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
	registerTriageNamespace(server)
	registerClusterHealthCheck(server)
	registerReviewWarnings(server)
}

// contextArg is the optional context argument shared by every prompt.
var contextArg = &mcp.PromptArgument{
	Name:        "context",
	Description: "kubeconfig context to target (optional)",
	Required:    false,
}

// userPrompt registers a prompt whose handler renders a single user message.
// render returns a short title and the message body.
func userPrompt(server *mcp.Server, p *mcp.Prompt, render func(args map[string]string) (title, body string, err error)) {
	server.AddPrompt(p, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		title, body, err := render(req.Params.Arguments)
		if err != nil {
			return nil, err
		}
		return &mcp.GetPromptResult{
			Description: title,
			Messages: []*mcp.PromptMessage{
				{Role: "user", Content: &mcp.TextContent{Text: body}},
			},
		}, nil
	})
}

// contextNote renders a " (context "x")" suffix when a context was given.
func contextNote(args map[string]string) string {
	if c := args["context"]; c != "" {
		return fmt.Sprintf(" (context %q)", c)
	}
	return ""
}

func registerDiagnosePod(server *mcp.Server) {
	userPrompt(server, &mcp.Prompt{
		Name:        "diagnose_pod",
		Description: "Diagnose why a pod is unhealthy and suggest a fix, using the kubeaid tools.",
		Arguments: []*mcp.PromptArgument{
			{Name: "namespace", Description: "namespace of the pod", Required: true},
			{Name: "pod_name", Description: "name of the pod", Required: true},
			contextArg,
		},
	}, func(args map[string]string) (string, string, error) {
		ns, pod := args["namespace"], args["pod_name"]
		if ns == "" || pod == "" {
			return "", "", fmt.Errorf("namespace and pod_name are required")
		}
		body := fmt.Sprintf(`Diagnose why pod %q in namespace %q%s is unhealthy.

Work through it methodically:
1. Call describe_pod to inspect its phase, container states (waiting/termination reasons), and recent events.
2. If a container is crashing or restarting, call get_pod_logs with previous=true to read the crashed instance's logs.
3. Identify the root cause and explain it in plain language, then suggest a concrete fix.`,
			pod, ns, contextNote(args))
		return fmt.Sprintf("Diagnose pod %s/%s", ns, pod), body, nil
	})
}

func registerTriageNamespace(server *mcp.Server) {
	userPrompt(server, &mcp.Prompt{
		Name:        "triage_namespace",
		Description: "Triage the health of a whole namespace: find unhealthy workloads and investigate each.",
		Arguments: []*mcp.PromptArgument{
			{Name: "namespace", Description: "namespace to triage", Required: true},
			contextArg,
		},
	}, func(args map[string]string) (string, string, error) {
		ns := args["namespace"]
		if ns == "" {
			return "", "", fmt.Errorf("namespace is required")
		}
		body := fmt.Sprintf(`Triage the health of namespace %q%s.

1. Call list_pods for the namespace and identify every pod that is not Running/Ready (e.g. CrashLoopBackOff, Pending, ImagePullBackOff, Error).
2. For each unhealthy pod, call describe_pod and, if it is crashing, get_pod_logs with previous=true.
3. Call list_deployments and flag any deployment whose ready count is below desired.
4. Summarize the namespace's health: what is broken, the likely root cause of each issue, and a recommended fix. If everything is healthy, say so clearly.`,
			ns, contextNote(args))
		return fmt.Sprintf("Triage namespace %s", ns), body, nil
	})
}

func registerClusterHealthCheck(server *mcp.Server) {
	userPrompt(server, &mcp.Prompt{
		Name:        "cluster_health_check",
		Description: "Run a high-level health check across the whole cluster.",
		Arguments:   []*mcp.PromptArgument{contextArg},
	}, func(args map[string]string) (string, string, error) {
		body := fmt.Sprintf(`Perform a health check of the cluster%s.

1. Call list_nodes and flag any node that is not Ready.
2. Call list_pods without a namespace to scan every namespace, and collect pods that are not Running/Ready.
3. For the most concerning pods, call describe_pod to understand why.
4. Call get_events to surface recent Warning events cluster-wide.
5. Give a concise health report: overall status, any unhealthy nodes/pods, the likely causes, and what to look at next. If the cluster is healthy, say so.`,
			contextNote(args))
		return "Cluster health check", body, nil
	})
}

func registerReviewWarnings(server *mcp.Server) {
	userPrompt(server, &mcp.Prompt{
		Name:        "review_warnings",
		Description: "Review recent Warning events and explain what they mean.",
		Arguments: []*mcp.PromptArgument{
			{Name: "namespace", Description: "namespace to review; omit for all namespaces", Required: false},
			contextArg,
		},
	}, func(args map[string]string) (string, string, error) {
		scope, nsClause := "all namespaces", ""
		if ns := args["namespace"]; ns != "" {
			scope = fmt.Sprintf("namespace %q", ns)
			nsClause = fmt.Sprintf(" with namespace %q", ns)
		}
		body := fmt.Sprintf(`Review recent events in %s%s.

1. Call get_events%s and focus on Warning-type events.
2. Group related warnings (e.g. repeated BackOff, FailedScheduling, image pull errors) and explain in plain language what each group indicates.
3. For any resource with serious or repeated warnings, investigate further (describe_pod / describe_resource) to pin down the cause.
4. Summarize the issues by severity and recommend concrete next steps. If there are no concerning warnings, say it looks healthy.`,
			scope, contextNote(args), nsClause)
		return fmt.Sprintf("Review warnings in %s", scope), body, nil
	})
}
