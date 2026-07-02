package tools

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestNodeRoles(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{"none", map[string]string{"kubernetes.io/hostname": "n1"}, "<none>"},
		{"control-plane", map[string]string{"node-role.kubernetes.io/control-plane": ""}, "control-plane"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &corev1.Node{}
			n.Labels = tt.labels
			if got := nodeRoles(n); got != tt.want {
				t.Errorf("nodeRoles() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNodeStatus(t *testing.T) {
	ready := &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
		{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
	}}}
	if got := nodeStatus(ready); got != "Ready" {
		t.Errorf("nodeStatus(ready) = %q, want Ready", got)
	}
	notReady := &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
		{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
	}}}
	if got := nodeStatus(notReady); got != "NotReady" {
		t.Errorf("nodeStatus(notReady) = %q, want NotReady", got)
	}
	unknown := &corev1.Node{}
	if got := nodeStatus(unknown); got != "Unknown" {
		t.Errorf("nodeStatus(unknown) = %q, want Unknown", got)
	}
}
