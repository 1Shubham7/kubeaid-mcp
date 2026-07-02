package tools

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMatchesKind(t *testing.T) {
	deploy := metav1.APIResource{
		Name:         "deployments",
		SingularName: "deployment",
		Kind:         "Deployment",
		ShortNames:   []string{"deploy"},
	}
	for _, in := range []string{"Deployment", "deployment", "deployments", "deploy", "DEPLOY"} {
		if !matchesKind(deploy, in) {
			t.Errorf("matchesKind(deploy, %q) = false, want true", in)
		}
	}
	for _, in := range []string{"pod", "service", "deploymentss"} {
		if matchesKind(deploy, in) {
			t.Errorf("matchesKind(deploy, %q) = true, want false", in)
		}
	}
}

func TestPruneMetadata(t *testing.T) {
	obj := map[string]any{
		"metadata": map[string]any{
			"name":          "nginx",
			"managedFields": []any{map[string]any{"manager": "kubectl"}},
			"annotations": map[string]any{
				"kubectl.kubernetes.io/last-applied-configuration": "{...}",
				"keep-me": "yes",
			},
		},
	}
	pruneMetadata(obj)
	meta := obj["metadata"].(map[string]any)
	if _, ok := meta["managedFields"]; ok {
		t.Error("managedFields was not removed")
	}
	ann := meta["annotations"].(map[string]any)
	if _, ok := ann["kubectl.kubernetes.io/last-applied-configuration"]; ok {
		t.Error("last-applied-configuration annotation was not removed")
	}
	if ann["keep-me"] != "yes" {
		t.Error("unrelated annotation was dropped")
	}
}

func TestPruneMetadataDropsEmptyAnnotations(t *testing.T) {
	obj := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				"kubectl.kubernetes.io/last-applied-configuration": "{...}",
			},
		},
	}
	pruneMetadata(obj)
	meta := obj["metadata"].(map[string]any)
	if _, ok := meta["annotations"]; ok {
		t.Error("annotations map should be removed once empty")
	}
}
