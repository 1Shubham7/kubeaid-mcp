package k8s

import (
	"fmt"
	"sort"
	"sync"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// ClientManager resolves and caches Kubernetes clients per kubeconfig context,
// so a single running server can serve any cluster in the kubeconfig.
type ClientManager struct {
	loadingRules   *clientcmd.ClientConfigLoadingRules
	rawConfig      clientcmdapi.Config
	defaultContext string

	mu    sync.Mutex
	cache map[string]*clientBundle
}

// clientBundle holds the clients built from one context's rest.Config. Building
// them does no network I/O; a connection is only made when a client is used.
type clientBundle struct {
	clientset kubernetes.Interface
	dynamic   dynamic.Interface
}

// ContextInfo describes one kubeconfig context.
type ContextInfo struct {
	Name      string `json:"name"`
	Cluster   string `json:"cluster"`
	IsDefault bool   `json:"isDefault"`
}

// NewClientManager loads the kubeconfig (empty path uses the standard
// resolution: KUBECONFIG env, then ~/.kube/config). defaultContext is used when
// a tool call omits one; empty falls back to the kubeconfig's current-context.
func NewClientManager(kubeconfigPath, defaultContext string) (*ClientManager, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}

	rawConfig, err := loadingRules.Load()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	if defaultContext == "" {
		defaultContext = rawConfig.CurrentContext
	}
	if defaultContext == "" {
		return nil, fmt.Errorf("no context given and kubeconfig has no current-context")
	}
	if _, ok := rawConfig.Contexts[defaultContext]; !ok {
		return nil, fmt.Errorf("default context %q not found in kubeconfig", defaultContext)
	}

	return &ClientManager{
		loadingRules:   loadingRules,
		rawConfig:      *rawConfig,
		defaultContext: defaultContext,
		cache:          make(map[string]*clientBundle),
	}, nil
}

// DefaultContext is the context used when a tool call omits one.
func (m *ClientManager) DefaultContext() string { return m.defaultContext }

// Contexts lists every context in the kubeconfig, sorted by name.
func (m *ClientManager) Contexts() []ContextInfo {
	out := make([]ContextInfo, 0, len(m.rawConfig.Contexts))
	for name, ctx := range m.rawConfig.Contexts {
		out = append(out, ContextInfo{
			Name:      name,
			Cluster:   ctx.Cluster,
			IsDefault: name == m.defaultContext,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Clientset returns the typed clientset for the named context (empty = default).
func (m *ClientManager) Clientset(contextName string) (kubernetes.Interface, error) {
	b, err := m.bundle(contextName)
	if err != nil {
		return nil, err
	}
	return b.clientset, nil
}

// DynamicClient returns the dynamic (untyped) client for the named context,
// used to read arbitrary resource kinds including CRDs.
func (m *ClientManager) DynamicClient(contextName string) (dynamic.Interface, error) {
	b, err := m.bundle(contextName)
	if err != nil {
		return nil, err
	}
	return b.dynamic, nil
}

// bundle builds and caches the clients for a context on first use.
func (m *ClientManager) bundle(contextName string) (*clientBundle, error) {
	if contextName == "" {
		contextName = m.defaultContext
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if b, ok := m.cache[contextName]; ok {
		return b, nil
	}
	if _, ok := m.rawConfig.Contexts[contextName]; !ok {
		return nil, fmt.Errorf("context %q not found in kubeconfig", contextName)
	}

	overrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(m.loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("building config for context %q: %w", contextName, err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("building clientset for context %q: %w", contextName, err)
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("building dynamic client for context %q: %w", contextName, err)
	}

	b := &clientBundle{clientset: clientset, dynamic: dyn}
	m.cache[contextName] = b
	return b, nil
}
