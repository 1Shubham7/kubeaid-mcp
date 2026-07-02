package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

type deleteResourceInput struct {
	mutationInput
	Kind       string `json:"kind" jsonschema:"resource kind, plural, or short name (e.g. Pod, pods, po)"`
	Name       string `json:"name" jsonschema:"name of the resource to delete"`
	Namespace  string `json:"namespace,omitempty" jsonschema:"namespace for namespaced resources; defaults to \"default\""`
	APIVersion string `json:"api_version,omitempty" jsonschema:"optional apiVersion (e.g. apps/v1) to disambiguate the kind"`
}

type deleteResourceOutput struct {
	Deleted string `json:"deleted"`
	DryRun  bool   `json:"dryRun"`
}

func registerDeleteResource(server *mcp.Server, kc *k8s.ClientManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_resource",
		Description: "Delete a Kubernetes resource by kind and name. Destructive. Set dry_run=true to check what would be deleted without deleting it.",
		Annotations: destructive,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in deleteResourceInput) (*mcp.CallToolResult, deleteResourceOutput, error) {
		if err := kc.EnsureWritable(in.Context); err != nil {
			return nil, deleteResourceOutput{}, err
		}
		clientset, err := kc.Clientset(in.Context)
		if err != nil {
			return nil, deleteResourceOutput{}, err
		}
		dyn, err := kc.DynamicClient(in.Context)
		if err != nil {
			return nil, deleteResourceOutput{}, err
		}

		ri, err := resourceInterface(clientset.Discovery(), dyn, in.Kind, in.APIVersion, in.Namespace)
		if err != nil {
			return nil, deleteResourceOutput{}, err
		}

		err = ri.Delete(ctx, in.Name, metav1.DeleteOptions{DryRun: dryRunOpts(in.DryRun)})
		if err != nil {
			return nil, deleteResourceOutput{}, fmt.Errorf("deleting %s %q: %w", in.Kind, in.Name, err)
		}

		return nil, deleteResourceOutput{
			Deleted: fmt.Sprintf("%s/%s", in.Kind, in.Name),
			DryRun:  in.DryRun,
		}, nil
	})
}
