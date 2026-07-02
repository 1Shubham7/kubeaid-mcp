package tools

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

type getEventsInput struct {
	contextInput
	Namespace string `json:"namespace,omitempty" jsonschema:"namespace to get events from; omit for all namespaces"`
}

type clusterEventInfo struct {
	Namespace string `json:"namespace,omitempty"`
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Object    string `json:"object"`
	Message   string `json:"message"`
	Count     int32  `json:"count"`
	Age       string `json:"age"`
}

type getEventsOutput struct {
	Events []clusterEventInfo `json:"events"`
}

func registerGetEvents(server *mcp.Server, kc *k8s.ClientManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_events",
		Description: "List recent events in a namespace (or across all namespaces if omitted), sorted oldest to newest. Warning-type events surface cluster problems.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getEventsInput) (*mcp.CallToolResult, getEventsOutput, error) {
		clientset, err := kc.Clientset(in.Context)
		if err != nil {
			return nil, getEventsOutput{}, err
		}

		list, err := clientset.CoreV1().Events(in.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, getEventsOutput{}, fmt.Errorf("listing events: %w", err)
		}
		sort.Slice(list.Items, func(i, j int) bool {
			return eventTime(list.Items[i]).Before(eventTime(list.Items[j]))
		})

		out := getEventsOutput{Events: make([]clusterEventInfo, 0, len(list.Items))}
		for _, e := range list.Items {
			out.Events = append(out.Events, clusterEventInfo{
				Namespace: e.Namespace,
				Type:      e.Type,
				Reason:    e.Reason,
				Object:    fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
				Message:   e.Message,
				Count:     e.Count,
				Age:       duration.HumanDuration(time.Since(eventTime(e))),
			})
		}
		return nil, out, nil
	})
}
