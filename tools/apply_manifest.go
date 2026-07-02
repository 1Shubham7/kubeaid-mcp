package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/1shubham7/kubeaid-mcp/k8s"
)

const fieldManager = "kubeaid-mcp"

type applyManifestInput struct {
	mutationInput
	Manifest string `json:"manifest" jsonschema:"one or more Kubernetes resources as YAML or JSON; multiple YAML documents separated by --- are supported"`
}

type appliedResource struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

type applyManifestOutput struct {
	Applied []appliedResource `json:"applied"`
	DryRun  bool              `json:"dryRun"`
}

func registerApplyManifest(server *mcp.Server, kc *k8s.ClientManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "apply_manifest",
		Description: "Create or update Kubernetes resources from a YAML or JSON manifest via server-side apply. Handles any kind including CRDs. Use dry_run to preview.",
		Annotations: additiveIdempotent,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in applyManifestInput) (*mcp.CallToolResult, applyManifestOutput, error) {
		if err := kc.EnsureWritable(in.Context); err != nil {
			return nil, applyManifestOutput{}, err
		}
		clientset, err := kc.Clientset(in.Context)
		if err != nil {
			return nil, applyManifestOutput{}, err
		}
		dyn, err := kc.DynamicClient(in.Context)
		if err != nil {
			return nil, applyManifestOutput{}, err
		}

		objs, err := decodeManifest(in.Manifest)
		if err != nil {
			return nil, applyManifestOutput{}, err
		}

		out := applyManifestOutput{Applied: make([]appliedResource, 0, len(objs)), DryRun: in.DryRun}
		for _, obj := range objs {
			if obj.GetKind() == "" || obj.GetAPIVersion() == "" {
				return nil, applyManifestOutput{}, fmt.Errorf("manifest document missing kind or apiVersion")
			}
			if obj.GetName() == "" {
				return nil, applyManifestOutput{}, fmt.Errorf("%s document missing metadata.name", obj.GetKind())
			}

			ri, err := resourceInterface(clientset.Discovery(), dyn, obj.GetKind(), obj.GetAPIVersion(), obj.GetNamespace())
			if err != nil {
				return nil, applyManifestOutput{}, err
			}

			applied, err := ri.Apply(ctx, obj.GetName(), obj, metav1.ApplyOptions{
				FieldManager: fieldManager,
				Force:        true,
				DryRun:       dryRunOpts(in.DryRun),
			})
			if err != nil {
				return nil, applyManifestOutput{}, fmt.Errorf("applying %s %q: %w", obj.GetKind(), obj.GetName(), err)
			}

			out.Applied = append(out.Applied, appliedResource{
				Kind:      applied.GetKind(),
				Name:      applied.GetName(),
				Namespace: applied.GetNamespace(),
			})
		}
		return nil, out, nil
	})
}

// decodeManifest parses a YAML/JSON manifest that may contain multiple documents.
func decodeManifest(manifest string) ([]*unstructured.Unstructured, error) {
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(manifest)), 4096)
	var objs []*unstructured.Unstructured
	for {
		raw := map[string]any{}
		if err := decoder.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("parsing manifest: %w", err)
		}
		if len(raw) == 0 {
			continue // skip empty documents (e.g. trailing ---)
		}
		objs = append(objs, &unstructured.Unstructured{Object: raw})
	}
	if len(objs) == 0 {
		return nil, fmt.Errorf("manifest contained no resources")
	}
	return objs, nil
}
