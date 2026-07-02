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

type listPodsInput struct {
	contextInput
	Namespace string `json:"namespace,omitempty" jsonschema:"namespace to list pods in; omit to list pods across all namespaces"`
}

type podSummary struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Ready     string `json:"ready"`
	Restarts  int32  `json:"restarts"`
	Age       string `json:"age"`
	Node      string `json:"node,omitempty"`
}

type listPodsOutput struct {
	Pods []podSummary `json:"pods"`
}

func registerListPods(server *mcp.Server, kc *k8s.ClientManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_pods",
		Description: "List pods in a namespace (or across all namespaces if omitted), with derived status, ready count, restarts and age.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listPodsInput) (*mcp.CallToolResult, listPodsOutput, error) {
		clientset, err := kc.Clientset(in.Context)
		if err != nil {
			return nil, listPodsOutput{}, err
		}

		list, err := clientset.CoreV1().Pods(in.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, listPodsOutput{}, fmt.Errorf("listing pods: %w", err)
		}

		out := listPodsOutput{Pods: make([]podSummary, 0, len(list.Items))}
		for i := range list.Items {
			pod := &list.Items[i]
			ready, total := readyContainers(pod)
			out.Pods = append(out.Pods, podSummary{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				Status:    podDisplayStatus(pod),
				Ready:     fmt.Sprintf("%d/%d", ready, total),
				Restarts:  totalRestarts(pod),
				Age:       duration.HumanDuration(time.Since(pod.CreationTimestamp.Time)),
				Node:      pod.Spec.NodeName,
			})
		}
		return nil, out, nil
	})
}
