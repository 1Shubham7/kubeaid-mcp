package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

type rolloutRestartInput struct {
	mutationInput
	Namespace string `json:"namespace" jsonschema:"namespace of the workload"`
	Name      string `json:"name" jsonschema:"name of the workload"`
	Kind      string `json:"kind,omitempty" jsonschema:"workload kind: deployment (default), statefulset, or daemonset"`
}

type rolloutRestartOutput struct {
	Restarted string `json:"restarted"`
	DryRun    bool   `json:"dryRun"`
}

func registerRolloutRestart(server *mcp.Server, kc *k8s.ClientManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "rollout_restart",
		Description: "Trigger a rolling restart of a deployment, statefulset, or daemonset by touching its pod template, causing pods to be recreated.",
		Annotations: additive,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in rolloutRestartInput) (*mcp.CallToolResult, rolloutRestartOutput, error) {
		if err := kc.EnsureWritable(in.Context); err != nil {
			return nil, rolloutRestartOutput{}, err
		}
		clientset, err := kc.Clientset(in.Context)
		if err != nil {
			return nil, rolloutRestartOutput{}, err
		}
		dyn, err := kc.DynamicClient(in.Context)
		if err != nil {
			return nil, rolloutRestartOutput{}, err
		}

		kind := in.Kind
		if kind == "" {
			kind = "deployment"
		}
		ri, err := resourceInterface(clientset.Discovery(), dyn, kind, "apps/v1", in.Namespace)
		if err != nil {
			return nil, rolloutRestartOutput{}, err
		}

		// Same mechanism as `kubectl rollout restart`: stamp a restart timestamp
		// on the pod template, which forces the controller to roll pods.
		patch := fmt.Sprintf(
			`{"spec":{"template":{"metadata":{"annotations":{"kubeaid-mcp.dev/restartedAt":%q}}}}}`,
			time.Now().Format(time.RFC3339),
		)
		_, err = ri.Patch(ctx, in.Name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{DryRun: dryRunOpts(in.DryRun)})
		if err != nil {
			return nil, rolloutRestartOutput{}, fmt.Errorf("restarting %s %q: %w", kind, in.Name, err)
		}

		return nil, rolloutRestartOutput{
			Restarted: fmt.Sprintf("%s/%s", kind, in.Name),
			DryRun:    in.DryRun,
		}, nil
	})
}
