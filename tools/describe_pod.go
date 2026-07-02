package tools

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

// eventTime resolves the timestamp of an event, which lives in different fields
// depending on how it was recorded.
func eventTime(e corev1.Event) time.Time {
	if !e.LastTimestamp.Time.IsZero() {
		return e.LastTimestamp.Time
	}
	return e.EventTime.Time
}

type describePodInput struct {
	contextInput
	Namespace string `json:"namespace" jsonschema:"namespace of the pod"`
	PodName   string `json:"pod_name" jsonschema:"name of the pod"`
}

type containerInfo struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount"`
	State        string `json:"state"`
	Reason       string `json:"reason,omitempty"`
	Message      string `json:"message,omitempty"`
	LastState    string `json:"lastState,omitempty"`
}

type eventInfo struct {
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Count   int32  `json:"count"`
	Age     string `json:"age"`
}

type describePodOutput struct {
	Name       string          `json:"name"`
	Namespace  string          `json:"namespace"`
	Node       string          `json:"node,omitempty"`
	Status     string          `json:"status"`
	PodIP      string          `json:"podIP,omitempty"`
	Age        string          `json:"age"`
	Containers []containerInfo `json:"containers"`
	Events     []eventInfo     `json:"events"`
}

func registerDescribePod(server *mcp.Server, kc *k8s.ClientManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "describe_pod",
		Description: "Describe a pod: its status, per-container state (including waiting/termination reasons and last restart), and recent events. Use this to diagnose why a pod is not healthy.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in describePodInput) (*mcp.CallToolResult, describePodOutput, error) {
		clientset, err := kc.Clientset(in.Context)
		if err != nil {
			return nil, describePodOutput{}, err
		}

		pod, err := clientset.CoreV1().Pods(in.Namespace).Get(ctx, in.PodName, metav1.GetOptions{})
		if err != nil {
			return nil, describePodOutput{}, fmt.Errorf("getting pod: %w", err)
		}

		out := describePodOutput{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Node:      pod.Spec.NodeName,
			Status:    podDisplayStatus(pod),
			PodIP:     pod.Status.PodIP,
			Age:       duration.HumanDuration(time.Since(pod.CreationTimestamp.Time)),
		}

		for _, cs := range pod.Status.ContainerStatuses {
			state, reason, message := containerState(cs)
			ci := containerInfo{
				Name:         cs.Name,
				Image:        cs.Image,
				Ready:        cs.Ready,
				RestartCount: cs.RestartCount,
				State:        state,
				Reason:       reason,
				Message:      message,
			}
			if t := cs.LastTerminationState.Terminated; t != nil {
				ci.LastState = fmt.Sprintf("Terminated: %s (exit code %d)", t.Reason, t.ExitCode)
			}
			out.Containers = append(out.Containers, ci)
		}

		events, err := clientset.CoreV1().Events(in.Namespace).List(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s", in.PodName),
		})
		if err != nil {
			return nil, describePodOutput{}, fmt.Errorf("listing events: %w", err)
		}
		sort.Slice(events.Items, func(i, j int) bool {
			return eventTime(events.Items[i]).Before(eventTime(events.Items[j]))
		})
		for _, e := range events.Items {
			out.Events = append(out.Events, eventInfo{
				Type:    e.Type,
				Reason:  e.Reason,
				Message: e.Message,
				Count:   e.Count,
				Age:     duration.HumanDuration(time.Since(eventTime(e))),
			})
		}

		return nil, out, nil
	})
}
