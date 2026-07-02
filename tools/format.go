package tools

import (
	corev1 "k8s.io/api/core/v1"
)

// podDisplayStatus derives the status a human expects to see (e.g.
// CrashLoopBackOff, ImagePullBackOff, Completed), which is not simply
// pod.Status.Phase. This mirrors a subset of what `kubectl get pods` computes.
func podDisplayStatus(pod *corev1.Pod) string {
	status := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		status = pod.Status.Reason
	}
	for _, cs := range pod.Status.ContainerStatuses {
		switch {
		case cs.State.Waiting != nil && cs.State.Waiting.Reason != "":
			status = cs.State.Waiting.Reason
		case cs.State.Terminated != nil && cs.State.Terminated.Reason != "":
			status = cs.State.Terminated.Reason
		}
	}
	if pod.DeletionTimestamp != nil {
		status = "Terminating"
	}
	return status
}

func readyContainers(pod *corev1.Pod) (ready, total int) {
	total = len(pod.Status.ContainerStatuses)
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
	}
	return ready, total
}

func totalRestarts(pod *corev1.Pod) int32 {
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}
	return restarts
}

// containerState reduces a container's state to a phase plus, when relevant, a
// reason and message (the fields that explain why a container isn't running).
func containerState(cs corev1.ContainerStatus) (state, reason, message string) {
	switch {
	case cs.State.Running != nil:
		return "Running", "", ""
	case cs.State.Waiting != nil:
		return "Waiting", cs.State.Waiting.Reason, cs.State.Waiting.Message
	case cs.State.Terminated != nil:
		return "Terminated", cs.State.Terminated.Reason, cs.State.Terminated.Message
	default:
		return "Unknown", "", ""
	}
}
