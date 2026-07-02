package k8s

import (
	"fmt"
	"sort"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// ClientManager resolves and caches one Kubernetes clientset per kubeconfig
// context, so a single running server can serve any cluster in the kubeconfig.
type ClientManager struct {
	loadingRules   *clientcmd.ClientConfigLoadingRules
	rawConfig      clientcmdapi.Config
	defaultContext string

	mu    sync.Mutex
	cache map[string]kubernetes.Interface
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
		cache:          make(map[string]kubernetes.Interface),
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

// Clientset returns a clientset for the named context, building and caching it
// on first use. An empty name uses the default context. The clientset connects
// lazily, so an unreachable cluster only errors when a tool actually calls it.
func (m *ClientManager) Clientset(contextName string) (kubernetes.Interface, error) {
	if contextName == "" {
		contextName = m.defaultContext
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if cs, ok := m.cache[contextName]; ok {
		return cs, nil
	}
	if _, ok := m.rawConfig.Contexts[contextName]; !ok {
		return nil, fmt.Errorf("context %q not found in kubeconfig", contextName)
	}

	overrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(m.loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("building config for context %q: %w", contextName, err)
	}

	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("building clientset for context %q: %w", contextName, err)
	}

	m.cache[contextName] = cs
	return cs, nil
}
