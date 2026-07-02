package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

type listNodesInput struct {
	contextInput
}

type nodeSummary struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Roles      string `json:"roles"`
	Age        string `json:"age"`
	Version    string `json:"version"`
	InternalIP string `json:"internalIP,omitempty"`
	OS         string `json:"os,omitempty"`
}

type listNodesOutput struct {
	Nodes []nodeSummary `json:"nodes"`
}

func registerListNodes(server *mcp.Server, kc *k8s.ClientManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_nodes",
		Description: "List cluster nodes with Ready status, roles, age, kubelet version and internal IP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listNodesInput) (*mcp.CallToolResult, listNodesOutput, error) {
		clientset, err := kc.Clientset(in.Context)
		if err != nil {
			return nil, listNodesOutput{}, err
		}

		list, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, listNodesOutput{}, fmt.Errorf("listing nodes: %w", err)
		}

		out := listNodesOutput{Nodes: make([]nodeSummary, 0, len(list.Items))}
		for i := range list.Items {
			n := &list.Items[i]
			out.Nodes = append(out.Nodes, nodeSummary{
				Name:       n.Name,
				Status:     nodeStatus(n),
				Roles:      nodeRoles(n),
				Age:        duration.HumanDuration(time.Since(n.CreationTimestamp.Time)),
				Version:    n.Status.NodeInfo.KubeletVersion,
				InternalIP: nodeInternalIP(n),
				OS:         n.Status.NodeInfo.OSImage,
			})
		}
		return nil, out, nil
	})
}

func nodeStatus(n *corev1.Node) string {
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady {
			if c.Status == corev1.ConditionTrue {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}

func nodeRoles(n *corev1.Node) string {
	var roles []string
	for label := range n.Labels {
		if role, ok := strings.CutPrefix(label, "node-role.kubernetes.io/"); ok && role != "" {
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return "<none>"
	}
	return strings.Join(roles, ",")
}

func nodeInternalIP(n *corev1.Node) string {
	for _, addr := range n.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}
	return ""
}
