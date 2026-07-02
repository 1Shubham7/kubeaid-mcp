package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

// dryRunOpts returns the DryRun value for a request: server-side dry-run when
// dry is true, nil otherwise.
func dryRunOpts(dry bool) []string {
	if dry {
		return []string{metav1.DryRunAll}
	}
	return nil
}

// mutationInput is the common shape for write tools: which context, and whether
// to only simulate the change.
type mutationInput struct {
	contextInput
	DryRun bool `json:"dry_run,omitempty" jsonschema:"if true, simulate the change server-side without persisting it"`
}

// contextInput is embedded in every tool's input to let a caller target a
// specific kubeconfig context. Omitting it uses the server's default context.
type contextInput struct {
	Context string `json:"context,omitempty" jsonschema:"kubeconfig context (cluster) to target; omit to use the default context"`
}

func boolPtr(b bool) *bool { return &b }

// Tool annotations tell clients how risky a tool is, so they can prompt
// appropriately (e.g. confirm before destructive actions).
var (
	readOnly           = &mcp.ToolAnnotations{ReadOnlyHint: true}
	additiveIdempotent = &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true}
	additive           = &mcp.ToolAnnotations{DestructiveHint: boolPtr(false)}
	destructive        = &mcp.ToolAnnotations{DestructiveHint: boolPtr(true)}
)

// resourceInterface resolves a kind (and optional apiVersion) to a dynamic
// resource interface, scoped to namespace when the resource is namespaced.
func resourceInterface(disc discovery.DiscoveryInterface, dyn dynamic.Interface, kind, apiVersion, namespace string) (dynamic.ResourceInterface, error) {
	gvr, namespaced, err := resolveResource(disc, kind, apiVersion)
	if err != nil {
		return nil, err
	}
	if namespaced {
		ns := namespace
		if ns == "" {
			ns = "default"
		}
		return dyn.Resource(gvr).Namespace(ns), nil
	}
	return dyn.Resource(gvr), nil
}
