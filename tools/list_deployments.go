package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

type listDeploymentsInput struct {
	contextInput
	Namespace string `json:"namespace,omitempty" jsonschema:"namespace to list deployments in; omit to list across all namespaces"`
}

type deploymentSummary struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Ready     string `json:"ready"`
	UpToDate  int32  `json:"upToDate"`
	Available int32  `json:"available"`
	Age       string `json:"age"`
}

type listDeploymentsOutput struct {
	Deployments []deploymentSummary `json:"deployments"`
}

func registerListDeployments(server *mcp.Server, kc *k8s.ClientManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_deployments",
		Description: "List deployments in a namespace (or across all namespaces if omitted), with ready/up-to-date/available replica counts and age.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listDeploymentsInput) (*mcp.CallToolResult, listDeploymentsOutput, error) {
		clientset, err := kc.Clientset(in.Context)
		if err != nil {
			return nil, listDeploymentsOutput{}, err
		}

		list, err := clientset.AppsV1().Deployments(in.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, listDeploymentsOutput{}, fmt.Errorf("listing deployments: %w", err)
		}

		out := listDeploymentsOutput{Deployments: make([]deploymentSummary, 0, len(list.Items))}
		for i := range list.Items {
			d := &list.Items[i]
			desired := int32(0)
			if d.Spec.Replicas != nil {
				desired = *d.Spec.Replicas
			}
			out.Deployments = append(out.Deployments, deploymentSummary{
				Namespace: d.Namespace,
				Name:      d.Name,
				Ready:     fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, desired),
				UpToDate:  d.Status.UpdatedReplicas,
				Available: d.Status.AvailableReplicas,
				Age:       duration.HumanDuration(time.Since(d.CreationTimestamp.Time)),
			})
		}
		return nil, out, nil
	})
}
