package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

type listNamespacesInput struct {
	contextInput
}

type namespaceInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type listNamespacesOutput struct {
	Namespaces []namespaceInfo `json:"namespaces"`
}

func registerListNamespaces(server *mcp.Server, kc *k8s.ClientManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_namespaces",
		Description: "List all namespaces in the Kubernetes cluster, with their status.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listNamespacesInput) (*mcp.CallToolResult, listNamespacesOutput, error) {
		clientset, err := kc.Clientset(in.Context)
		if err != nil {
			return nil, listNamespacesOutput{}, err
		}

		list, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, listNamespacesOutput{}, fmt.Errorf("listing namespaces: %w", err)
		}

		out := listNamespacesOutput{Namespaces: make([]namespaceInfo, 0, len(list.Items))}
		for _, ns := range list.Items {
			out.Namespaces = append(out.Namespaces, namespaceInfo{
				Name:   ns.Name,
				Status: string(ns.Status.Phase),
			})
		}
		return nil, out, nil
	})
}
