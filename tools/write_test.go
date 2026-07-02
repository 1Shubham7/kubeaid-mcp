package tools

import (
	"testing"

	"k8s.io/apimachinery/pkg/types"
)

func TestParsePatchType(t *testing.T) {
	cases := map[string]types.PatchType{
		"":          types.StrategicMergePatchType,
		"strategic": types.StrategicMergePatchType,
		"merge":     types.MergePatchType,
		"json":      types.JSONPatchType,
	}
	for in, want := range cases {
		got, err := parsePatchType(in)
		if err != nil || got != want {
			t.Errorf("parsePatchType(%q) = (%v, %v), want (%v, nil)", in, got, err, want)
		}
	}
	if _, err := parsePatchType("bogus"); err == nil {
		t.Error("parsePatchType(bogus) should error")
	}
}

func TestDecodeManifest(t *testing.T) {
	multi := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: a
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: b
`
	objs, err := decodeManifest(multi)
	if err != nil {
		t.Fatalf("decodeManifest error: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("got %d objects, want 2", len(objs))
	}
	if objs[0].GetName() != "a" || objs[1].GetName() != "b" {
		t.Errorf("names = %q, %q; want a, b", objs[0].GetName(), objs[1].GetName())
	}

	if _, err := decodeManifest("   \n"); err == nil {
		t.Error("decodeManifest of empty input should error")
	}
}
