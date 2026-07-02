package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

type patchResourceInput struct {
	mutationInput
	Kind       string `json:"kind" jsonschema:"resource kind, plural, or short name (e.g. Deployment, deploy)"`
	Name       string `json:"name" jsonschema:"name of the resource to patch"`
	Namespace  string `json:"namespace,omitempty" jsonschema:"namespace for namespaced resources; defaults to \"default\""`
	APIVersion string `json:"api_version,omitempty" jsonschema:"optional apiVersion (e.g. apps/v1) to disambiguate the kind"`
	Patch      string `json:"patch" jsonschema:"the patch body as JSON"`
	PatchType  string `json:"patch_type,omitempty" jsonschema:"patch type: strategic (default), merge, or json"`
}

type patchResourceOutput struct {
	Patched string `json:"patched"`
	DryRun  bool   `json:"dryRun"`
}

func registerPatchResource(server *mcp.Server, kc *k8s.ClientManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "patch_resource",
		Description: "Apply a patch to an existing resource. patch_type is strategic (default, for built-in kinds), merge, or json. Use dry_run to preview.",
		Annotations: additive,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in patchResourceInput) (*mcp.CallToolResult, patchResourceOutput, error) {
		if err := kc.EnsureWritable(in.Context); err != nil {
			return nil, patchResourceOutput{}, err
		}
		patchType, err := parsePatchType(in.PatchType)
		if err != nil {
			return nil, patchResourceOutput{}, err
		}
		clientset, err := kc.Clientset(in.Context)
		if err != nil {
			return nil, patchResourceOutput{}, err
		}
		dyn, err := kc.DynamicClient(in.Context)
		if err != nil {
			return nil, patchResourceOutput{}, err
		}

		ri, err := resourceInterface(clientset.Discovery(), dyn, in.Kind, in.APIVersion, in.Namespace)
		if err != nil {
			return nil, patchResourceOutput{}, err
		}

		_, err = ri.Patch(ctx, in.Name, patchType, []byte(in.Patch), metav1.PatchOptions{DryRun: dryRunOpts(in.DryRun)})
		if err != nil {
			return nil, patchResourceOutput{}, fmt.Errorf("patching %s %q: %w", in.Kind, in.Name, err)
		}

		return nil, patchResourceOutput{
			Patched: fmt.Sprintf("%s/%s", in.Kind, in.Name),
			DryRun:  in.DryRun,
		}, nil
	})
}

func parsePatchType(s string) (types.PatchType, error) {
	switch s {
	case "", "strategic":
		return types.StrategicMergePatchType, nil
	case "merge":
		return types.MergePatchType, nil
	case "json":
		return types.JSONPatchType, nil
	default:
		return "", fmt.Errorf("invalid patch_type %q: want strategic, merge, or json", s)
	}
}
