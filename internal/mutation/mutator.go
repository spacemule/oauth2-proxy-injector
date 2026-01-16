package mutation

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/spacemule/oauth2-proxy-injector/internal/annotation"
	"github.com/spacemule/oauth2-proxy-injector/internal/config"
)

// SidecarContainerName is the name used for the injected oauth2-proxy container
const SidecarContainerName = "oauth2-proxy"

// InjectedAnnotation is added to pods that have been mutated
// Used to prevent double-injection and for debugging
const InjectedAnnotation = "spacemule.net/oauth2-proxy.injected"

// Mutator defines the contract for pod mutation operations
type Mutator interface {
	// Mutate takes a pod and returns JSON patch operations to inject oauth2-proxy
	Mutate(ctx context.Context, pod *corev1.Pod) ([]PatchOperation, error)
}

// PodMutator implements Mutator for oauth2-proxy sidecar injection
type PodMutator struct {
	annotationParser annotation.Parser
	configLoader     config.Loader
	sidecarBuilder   SidecarBuilder
}

// NewPodMutator creates a new PodMutator with its dependencies
func NewPodMutator(
	parser annotation.Parser,
	loader config.Loader,
	builder SidecarBuilder,
) *PodMutator {
	return &PodMutator{
		annotationParser: parser,
		configLoader: loader,
		sidecarBuilder: builder,
	}
}

// Mutate inspects pod annotations and injects oauth2-proxy sidecar if enabled
func (m *PodMutator) Mutate(ctx context.Context, pod *corev1.Pod) ([]PatchOperation, error) {
	var ret []PatchOperation
	annotationCfg, err := m.annotationParser.Parse(pod.Annotations)
	if err != nil {
		return nil, err
	}
	if !annotationCfg.Enabled {
		return ret, nil
	}
	
	if isAlreadyInjected(pod) {
		return ret, nil
	}
	
	proxyCfg, err := m.configLoader.Load(ctx, annotationCfg.ConfigMapName, pod.Namespace)
	if err != nil {
		return nil, err
	}

	ports := collectContainerPorts(pod)
	mapping, err := CalculatePortMapping(ports, annotationCfg)
	if err != nil {
		return nil, err
	}
	
	container, volumes := m.sidecarBuilder.Build(proxyCfg, mapping, annotationCfg)

	patchBuilder := NewPatchBuilder(hasExistingAnnotations(pod), hasExistingLabels(pod), hasExistingVolumes(pod))
	patchBuilder.AddContainer(container)

	i, j, remove := findProtectedPort(pod, annotationCfg.ProtectedPort)
	if remove {
		patchBuilder.RemovePort(i, j)
	}
	
	for _, v := range volumes {
		patchBuilder.AddVolume(v)
	}

	return patchBuilder.AddAnnotation(InjectedAnnotation, "true").Build(), nil
}

// collectContainerPorts gathers all ports from all containers in the pod
func collectContainerPorts(pod *corev1.Pod) []corev1.ContainerPort {
	var ret []corev1.ContainerPort
	
	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			ret = append(ret, p)
		}
	}
	
	return ret
}

// findProtectedPort finds the container and port indices for the protected port
// Returns containerIndex, portIndex, found
func findProtectedPort(pod *corev1.Pod, portName string) (int, int, bool) {
	for i, c := range pod.Spec.Containers {
		for j, p := range c.Ports {
			if p.Name == portName {
				return i, j, true
			}
		}
	}
	return 0, 0, false
}

// isAlreadyInjected checks if the pod already has an oauth2-proxy sidecar
func isAlreadyInjected(pod *corev1.Pod) bool {
	for k := range pod.Annotations {
		if k == InjectedAnnotation {
			for _, c := range pod.Spec.Containers {
				if c.Name == SidecarContainerName {
					return true
				}
			}
		}
	}
	return false
}

// hasExistingAnnotations checks if the pod has any annotations
func hasExistingAnnotations(pod *corev1.Pod) bool {
	return len(pod.Annotations) > 0
}

// hasExistingLabels checks if the pod has any labels
func hasExistingLabels(pod *corev1.Pod) bool {
	return len(pod.Labels) > 0
}

// hasExistingVolumes checks if the pod has any volumes
func hasExistingVolumes(pod *corev1.Pod) bool {
	return len(pod.Spec.Volumes) > 0
}