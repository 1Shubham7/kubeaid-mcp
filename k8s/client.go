package k8s

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps a Kubernetes clientset used by the MCP tools.
type Client struct {
	Clientset kubernetes.Interface
}

// NewClient builds a clientset from a kubeconfig file. An empty kubeconfigPath
// falls back to the standard resolution (KUBECONFIG env var, then
// ~/.kube/config). An empty contextName uses the file's current-context.
func NewClient(kubeconfigPath, contextName string) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}

	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("building clientset: %w", err)
	}

	return &Client{Clientset: clientset}, nil
}
