package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

type scaleDeploymentInput struct {
	mutationInput
	Namespace string `json:"namespace" jsonschema:"namespace of the deployment"`
	Name      string `json:"name" jsonschema:"name of the deployment"`
	Replicas  int32  `json:"replicas" jsonschema:"desired number of replicas"`
}

type scaleDeploymentOutput struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	PreviousReplica int32  `json:"previousReplicas"`
	Replicas        int32  `json:"replicas"`
	DryRun          bool   `json:"dryRun"`
}

func registerScaleDeployment(server *mcp.Server, kc *k8s.ClientManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "scale_deployment",
		Description: "Set the replica count of a deployment.",
		Annotations: additiveIdempotent,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in scaleDeploymentInput) (*mcp.CallToolResult, scaleDeploymentOutput, error) {
		if err := kc.EnsureWritable(in.Context); err != nil {
			return nil, scaleDeploymentOutput{}, err
		}
		clientset, err := kc.Clientset(in.Context)
		if err != nil {
			return nil, scaleDeploymentOutput{}, err
		}

		deployments := clientset.AppsV1().Deployments(in.Namespace)
		scale, err := deployments.GetScale(ctx, in.Name, metav1.GetOptions{})
		if err != nil {
			return nil, scaleDeploymentOutput{}, fmt.Errorf("getting scale for deployment %q: %w", in.Name, err)
		}

		previous := scale.Spec.Replicas
		scale.Spec.Replicas = in.Replicas
		updated, err := deployments.UpdateScale(ctx, in.Name, scale, metav1.UpdateOptions{DryRun: dryRunOpts(in.DryRun)})
		if err != nil {
			return nil, scaleDeploymentOutput{}, fmt.Errorf("scaling deployment %q: %w", in.Name, err)
		}

		return nil, scaleDeploymentOutput{
			Name:            in.Name,
			Namespace:       in.Namespace,
			PreviousReplica: previous,
			Replicas:        updated.Spec.Replicas,
			DryRun:          in.DryRun,
		}, nil
	})
}
