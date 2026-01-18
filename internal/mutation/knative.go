package mutation

import (
	corev1 "k8s.io/api/core/v1"
)

// KnativeQueueProxyContainer is the name of Knative's sidecar
const KnativeQueueProxyContainer = "queue-proxy"

// KnativeUserPortEnv is the env var queue-proxy uses to know where to forward traffic
const KnativeUserPortEnv = "USER_PORT"

// KnativeDetector detects whether a pod is managed by Knative Serving
type KnativeDetector interface {
	// IsKnativePod returns true if the pod is a Knative Serving pod
	IsKnativePod(pod *corev1.Pod) bool

	// FindQueueProxyIndex returns the index of the queue-proxy container
	FindQueueProxyIndex(pod *corev1.Pod) (int, bool)
}

// DefaultKnativeDetector implements KnativeDetector
type DefaultKnativeDetector struct{}

// NewKnativeDetector creates a new DefaultKnativeDetector
func NewKnativeDetector() *DefaultKnativeDetector {
	return &DefaultKnativeDetector{}
}

// IsKnativePod checks if this pod is managed by Knative Serving
func (d *DefaultKnativeDetector) IsKnativePod(pod *corev1.Pod) bool {
	for k, _ := range pod.Labels {
		if k == "serving.knative.dev/revision" || k == "serving.knative.dev/service" {
			return true
		}
	}
	_, b := d.FindQueueProxyIndex(pod)
	if b {
		return true
	}
	return false
}

// FindQueueProxyIndex locates the queue-proxy container in the pod spec
func (d *DefaultKnativeDetector) FindQueueProxyIndex(pod *corev1.Pod) (int, bool) {
	for i, c := range pod.Spec.Containers {
		if c.Name == "queue-proxy" {
			return i, true
		}
	}
	return -1, false
}

// FindUserPortEnvIndex finds the USER_PORT env var in queue-proxy's env slice
// Returns the index of the env var, or -1 if not found
func FindUserPortEnvIndex(pod *corev1.Pod, queueProxyIndex int) int {
	for i, env := range pod.Spec.Containers[queueProxyIndex].Env {
		if env.Name == "USER_PORT" {
			return i
		}
	}
	return -1
}
