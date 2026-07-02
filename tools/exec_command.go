package tools

import (
	"bytes"
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

type execCommandInput struct {
	contextInput
	Namespace string   `json:"namespace" jsonschema:"namespace of the pod"`
	PodName   string   `json:"pod_name" jsonschema:"name of the pod"`
	Container string   `json:"container,omitempty" jsonschema:"container name; omit for a single-container pod"`
	Command   []string `json:"command" jsonschema:"command and arguments to run, e.g. [\"sh\", \"-c\", \"ls /\"]"`
}

type execCommandOutput struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

func registerExecCommand(server *mcp.Server, kc *k8s.ClientManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "exec_command",
		Description: "Run a command inside a pod's container and return its stdout and stderr. Powerful and potentially dangerous.",
		Annotations: destructive,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in execCommandInput) (*mcp.CallToolResult, execCommandOutput, error) {
		if err := kc.EnsureExecAllowed(in.Context); err != nil {
			return nil, execCommandOutput{}, err
		}
		if len(in.Command) == 0 {
			return nil, execCommandOutput{}, fmt.Errorf("command must not be empty")
		}
		clientset, err := kc.Clientset(in.Context)
		if err != nil {
			return nil, execCommandOutput{}, err
		}
		restConfig, err := kc.RESTConfig(in.Context)
		if err != nil {
			return nil, execCommandOutput{}, err
		}

		req := clientset.CoreV1().RESTClient().Post().
			Resource("pods").Name(in.PodName).Namespace(in.Namespace).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Container: in.Container,
				Command:   in.Command,
				Stdout:    true,
				Stderr:    true,
			}, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
		if err != nil {
			return nil, execCommandOutput{}, fmt.Errorf("setting up exec: %w", err)
		}

		var stdout, stderr bytes.Buffer
		streamErr := exec.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &stdout, Stderr: &stderr})

		out := execCommandOutput{Stdout: stdout.String(), Stderr: stderr.String()}
		if streamErr != nil {
			// A non-zero exit code surfaces here; return the captured output too.
			return nil, out, fmt.Errorf("command failed: %w", streamErr)
		}
		return nil, out, nil
	})
}
