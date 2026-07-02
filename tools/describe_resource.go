package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

type describeResourceInput struct {
	contextInput
	Kind       string `json:"kind" jsonschema:"resource kind, plural, or short name (e.g. Deployment, deployments, deploy)"`
	Name       string `json:"name" jsonschema:"name of the resource"`
	Namespace  string `json:"namespace,omitempty" jsonschema:"namespace for namespaced resources; defaults to \"default\""`
	APIVersion string `json:"api_version,omitempty" jsonschema:"optional apiVersion (e.g. apps/v1) to disambiguate a kind that exists in multiple API groups"`
}

type describeResourceOutput struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Object     map[string]any `json:"object"`
}

func registerDescribeResource(server *mcp.Server, kc *k8s.ClientManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "describe_resource",
		Description: "Fetch any Kubernetes resource by kind and name, including custom resources (CRDs). Returns the resource object with noise (managedFields, last-applied annotation) stripped.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in describeResourceInput) (*mcp.CallToolResult, describeResourceOutput, error) {
		clientset, err := kc.Clientset(in.Context)
		if err != nil {
			return nil, describeResourceOutput{}, err
		}
		dyn, err := kc.DynamicClient(in.Context)
		if err != nil {
			return nil, describeResourceOutput{}, err
		}

		gvr, namespaced, err := resolveResource(clientset.Discovery(), in.Kind, in.APIVersion)
		if err != nil {
			return nil, describeResourceOutput{}, err
		}

		var ri dynamic.ResourceInterface = dyn.Resource(gvr)
		if namespaced {
			ns := in.Namespace
			if ns == "" {
				ns = "default"
			}
			ri = dyn.Resource(gvr).Namespace(ns)
		}

		obj, err := ri.Get(ctx, in.Name, metav1.GetOptions{})
		if err != nil {
			return nil, describeResourceOutput{}, fmt.Errorf("getting %s %q: %w", in.Kind, in.Name, err)
		}

		content := obj.Object
		pruneMetadata(content)

		return nil, describeResourceOutput{
			APIVersion: obj.GetAPIVersion(),
			Kind:       obj.GetKind(),
			Object:     content,
		}, nil
	})
}

// resolveResource maps a user-supplied kind (or plural, or short name) to its
// GroupVersionResource and whether it is namespaced, by scanning the cluster's
// discovery data. An optional apiVersion narrows the search.
func resolveResource(disc discovery.DiscoveryInterface, kind, apiVersion string) (schema.GroupVersionResource, bool, error) {
	lists, err := disc.ServerPreferredResources()
	// ServerPreferredResources can return partial data with an error when some
	// API groups are unavailable; proceed as long as we got something.
	if len(lists) == 0 && err != nil {
		return schema.GroupVersionResource{}, false, fmt.Errorf("discovering API resources: %w", err)
	}

	for _, list := range lists {
		if apiVersion != "" && list.GroupVersion != apiVersion {
			continue
		}
		gv, perr := schema.ParseGroupVersion(list.GroupVersion)
		if perr != nil {
			continue
		}
		for _, r := range list.APIResources {
			if strings.Contains(r.Name, "/") {
				continue // subresource such as pods/log
			}
			if matchesKind(r, kind) {
				return gv.WithResource(r.Name), r.Namespaced, nil
			}
		}
	}
	return schema.GroupVersionResource{}, false, fmt.Errorf("no resource found matching kind %q", kind)
}

func matchesKind(r metav1.APIResource, kind string) bool {
	if strings.EqualFold(r.Kind, kind) ||
		strings.EqualFold(r.Name, kind) ||
		strings.EqualFold(r.SingularName, kind) {
		return true
	}
	for _, sn := range r.ShortNames {
		if strings.EqualFold(sn, kind) {
			return true
		}
	}
	return false
}

// pruneMetadata removes fields that bloat a resource without helping diagnosis.
func pruneMetadata(obj map[string]any) {
	meta, ok := obj["metadata"].(map[string]any)
	if !ok {
		return
	}
	delete(meta, "managedFields")
	if ann, ok := meta["annotations"].(map[string]any); ok {
		delete(ann, "kubectl.kubernetes.io/last-applied-configuration")
		if len(ann) == 0 {
			delete(meta, "annotations")
		}
	}
}
