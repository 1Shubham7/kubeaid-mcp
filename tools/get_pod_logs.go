package tools

import (
	"context"
	"fmt"
	"io"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

const defaultLogLines = 100

type getPodLogsInput struct {
	contextInput
	Namespace string `json:"namespace" jsonschema:"namespace of the pod"`
	PodName   string `json:"pod_name" jsonschema:"name of the pod"`
	Container string `json:"container,omitempty" jsonschema:"container name; omit for a single-container pod"`
	Lines     int64  `json:"lines,omitempty" jsonschema:"number of lines from the end of the log to return (default 100)"`
	Previous  bool   `json:"previous,omitempty" jsonschema:"return logs from the previous terminated instance of the container; useful for crash loops"`
}

type getPodLogsOutput struct {
	Logs string `json:"logs"`
}

func registerGetPodLogs(server *mcp.Server, kc *k8s.ClientManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_pod_logs",
		Description: "Fetch the tail of a pod's container logs. Set previous=true to read logs from a crashed container's prior instance.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getPodLogsInput) (*mcp.CallToolResult, getPodLogsOutput, error) {
		clientset, err := kc.Clientset(in.Context)
		if err != nil {
			return nil, getPodLogsOutput{}, err
		}

		lines := in.Lines
		if lines <= 0 {
			lines = defaultLogLines
		}

		req := clientset.CoreV1().Pods(in.Namespace).GetLogs(in.PodName, &corev1.PodLogOptions{
			Container: in.Container,
			TailLines: &lines,
			Previous:  in.Previous,
		})
		stream, err := req.Stream(ctx)
		if err != nil {
			return nil, getPodLogsOutput{}, fmt.Errorf("streaming logs: %w", err)
		}
		defer stream.Close()

		data, err := io.ReadAll(stream)
		if err != nil {
			return nil, getPodLogsOutput{}, fmt.Errorf("reading logs: %w", err)
		}

		return nil, getPodLogsOutput{Logs: string(data)}, nil
	})
}
