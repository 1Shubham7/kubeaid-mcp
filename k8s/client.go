package k8s

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Config configures a ClientManager.
type Config struct {
	// KubeconfigPath is the kubeconfig file; empty uses standard resolution
	// (KUBECONFIG env, then ~/.kube/config).
	KubeconfigPath string
	// DefaultContext is used when a tool call omits one; empty falls back to the
	// kubeconfig's current-context.
	DefaultContext string
	// RequestTimeout bounds every Kubernetes API call.
	RequestTimeout time.Duration
	// AllowWrites enables mutating tools. Off by default, keeping the server
	// read-only.
	AllowWrites bool
	// AllowExec enables the exec tool (running commands inside containers).
	AllowExec bool
	// ProtectedContexts may never be written to or exec'd into, even when
	// AllowWrites/AllowExec are set.
	ProtectedContexts []string
}

// ClientManager resolves and caches Kubernetes clients per kubeconfig context,
// so a single running server can serve any cluster in the kubeconfig.
type ClientManager struct {
	loadingRules   *clientcmd.ClientConfigLoadingRules
	rawConfig      clientcmdapi.Config
	defaultContext string
	requestTimeout time.Duration
	allowWrites    bool
	allowExec      bool
	protected      map[string]bool

	mu    sync.Mutex
	cache map[string]*clientBundle
}

// clientBundle holds the clients built from one context's rest.Config. Building
// them does no network I/O; a connection is only made when a client is used.
type clientBundle struct {
	restConfig *rest.Config
	clientset  kubernetes.Interface
	dynamic    dynamic.Interface
}

// ContextInfo describes one kubeconfig context.
type ContextInfo struct {
	Name      string `json:"name"`
	Cluster   string `json:"cluster"`
	IsDefault bool   `json:"isDefault"`
	Protected bool   `json:"protected,omitempty"`
}

// NewClientManager loads the kubeconfig and prepares to serve every context.
func NewClientManager(cfg Config) (*ClientManager, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if cfg.KubeconfigPath != "" {
		loadingRules.ExplicitPath = cfg.KubeconfigPath
	}

	rawConfig, err := loadingRules.Load()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	defaultContext := cfg.DefaultContext
	if defaultContext == "" {
		defaultContext = rawConfig.CurrentContext
	}
	if defaultContext == "" {
		return nil, fmt.Errorf("no context given and kubeconfig has no current-context")
	}
	if _, ok := rawConfig.Contexts[defaultContext]; !ok {
		return nil, fmt.Errorf("default context %q not found in kubeconfig", defaultContext)
	}

	protected := make(map[string]bool, len(cfg.ProtectedContexts))
	for _, c := range cfg.ProtectedContexts {
		if c != "" {
			protected[c] = true
		}
	}

	return &ClientManager{
		loadingRules:   loadingRules,
		rawConfig:      *rawConfig,
		defaultContext: defaultContext,
		requestTimeout: cfg.RequestTimeout,
		allowWrites:    cfg.AllowWrites,
		allowExec:      cfg.AllowExec,
		protected:      protected,
		cache:          make(map[string]*clientBundle),
	}, nil
}

// DefaultContext is the context used when a tool call omits one.
func (m *ClientManager) DefaultContext() string { return m.defaultContext }

// WritesEnabled reports whether mutating tools should be registered.
func (m *ClientManager) WritesEnabled() bool { return m.allowWrites }

// ExecEnabled reports whether the exec tool should be registered.
func (m *ClientManager) ExecEnabled() bool { return m.allowExec }

// EnsureWritable returns an error if writes are not permitted for the context.
func (m *ClientManager) EnsureWritable(contextName string) error {
	if contextName == "" {
		contextName = m.defaultContext
	}
	if !m.allowWrites {
		return fmt.Errorf("writes are disabled; start the server with --allow-writes to enable them")
	}
	if m.protected[contextName] {
		return fmt.Errorf("context %q is protected against writes", contextName)
	}
	return nil
}

// EnsureExecAllowed returns an error if exec is not permitted for the context.
func (m *ClientManager) EnsureExecAllowed(contextName string) error {
	if contextName == "" {
		contextName = m.defaultContext
	}
	if !m.allowExec {
		return fmt.Errorf("exec is disabled; start the server with --allow-exec to enable it")
	}
	if m.protected[contextName] {
		return fmt.Errorf("context %q is protected; exec is not allowed", contextName)
	}
	return nil
}

// Contexts lists every context in the kubeconfig, sorted by name.
func (m *ClientManager) Contexts() []ContextInfo {
	out := make([]ContextInfo, 0, len(m.rawConfig.Contexts))
	for name, ctx := range m.rawConfig.Contexts {
		out = append(out, ContextInfo{
			Name:      name,
			Cluster:   ctx.Cluster,
			IsDefault: name == m.defaultContext,
			Protected: m.protected[name],
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
// used to read and write arbitrary resource kinds including CRDs.
func (m *ClientManager) DynamicClient(contextName string) (dynamic.Interface, error) {
	b, err := m.bundle(contextName)
	if err != nil {
		return nil, err
	}
	return b.dynamic, nil
}

// RESTConfig returns the rest.Config for the named context, needed for
// lower-level operations such as exec.
func (m *ClientManager) RESTConfig(contextName string) (*rest.Config, error) {
	b, err := m.bundle(contextName)
	if err != nil {
		return nil, err
	}
	return b.restConfig, nil
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
	config.Timeout = m.requestTimeout

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("building clientset for context %q: %w", contextName, err)
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("building dynamic client for context %q: %w", contextName, err)
	}

	b := &clientBundle{restConfig: config, clientset: clientset, dynamic: dyn}
	m.cache[contextName] = b
	return b, nil
}
