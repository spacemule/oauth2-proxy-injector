package mutation

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

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
	annotationParser 		annotation.Parser
	configLoader     		config.Loader
	sidecarBuilder   		SidecarBuilder
	configMerger     		config.Merger
	knativeDetector  		KnativeDetector
	initContainerBuilder 	InitContainerBuilder

	// defaultConfigMap is the name of the default ConfigMap in the webhook's namespace
	// Used when pods don't specify spacemule.net/oauth2-proxy.config annotation
	defaultConfigMap string

	// defaultConfigNamespace is the namespace where the default ConfigMap lives
	// Typically the webhook's own namespace
	defaultConfigNamespace string
}

// NewPodMutator creates a new PodMutator with its dependencies
//
// Parameters:
//   - parser: parses pod annotations into Config
//   - loader: loads ProxyConfig from ConfigMaps
//   - builder: builds the oauth2-proxy sidecar container
//   - merger: merges ConfigMap settings with annotation overrides
//   - knativeDetector: detects Knative pods and locates queue-proxy
//   - defaultConfigMap: name of the default ConfigMap (e.g., "oauth2-proxy-config")
//   - defaultConfigNamespace: namespace of the default ConfigMap (webhook's namespace)
func NewPodMutator(
	parser annotation.Parser,
	loader config.Loader,
	builder SidecarBuilder,
	merger config.Merger,
	knativeDetector KnativeDetector,
	initContainerBuilder InitContainerBuilder,
	defaultConfigMap string,
	defaultConfigNamespace string,
) *PodMutator {
	return &PodMutator{
		annotationParser:       parser,
		configLoader:           loader,
		sidecarBuilder:         builder,
		configMerger:           merger,
		knativeDetector:        knativeDetector,
		initContainerBuilder:   initContainerBuilder,
		defaultConfigMap:       defaultConfigMap,
		defaultConfigNamespace: defaultConfigNamespace,
	}
}

// Mutate inspects pod annotations and injects oauth2-proxy sidecar if enabled
func (m *PodMutator) Mutate(ctx context.Context, pod *corev1.Pod) ([]PatchOperation, error) {
	var ret []PatchOperation
	var cm, cmNamespace string
	var proxyCfg *config.ProxyConfig

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

	if annotationCfg.ConfigMapName != "" {
		cm = annotationCfg.ConfigMapName
		cmNamespace = pod.Namespace
	} else if m.defaultConfigMap != "" {
		cm = m.defaultConfigMap
		cmNamespace = m.defaultConfigNamespace
	}

	if cm != "" {
		proxyCfg, err = m.configLoader.Load(ctx, cm, cmNamespace)
		if err != nil {
			return nil, err
		}
	} else {
		proxyCfg = config.NewEmptyProxyConfig()
	}

	effectiveCfg, err := m.configMerger.Merge(proxyCfg, annotationCfg)
	if err != nil {
		return nil, err
	}

	var mapping PortMapping
	if effectiveCfg.ProtectedPort != "" {
		ports := collectContainerPorts(pod)
		mapping, err = CalculatePortMapping(ports, effectiveCfg)
		if err != nil {
			return nil, err
		}
	}

	initContainer := m.initContainerBuilder.Build(effectiveCfg, mapping)
	container, volumes := m.sidecarBuilder.Build(effectiveCfg, mapping)

	patchBuilder := NewPatchBuilder(hasExistingAnnotations(pod), hasExistingLabels(pod), hasExistingVolumes(pod))
	patchBuilder.AddInitContainer(initContainer)
	patchBuilder.AddContainer(container)

	if annotation.IsNamedPort(effectiveCfg.ProtectedPort) {
		i, j, remove := findProtectedPort(pod, effectiveCfg.ProtectedPort)
		if remove {
			patchBuilder.RemovePort(i, j)
		}
		rewrites := rewriteProbePortNames(pod, effectiveCfg.ProtectedPort, mapping.ProxyPort)
		for _, rw := range rewrites {
			patchBuilder.ReplaceProbePort(rw.ContainerIndex, rw.ProbeType, rw.HandlerType, rw.NewPort)
		}
	}

	// When block-direct-access is enabled, rewrite health checks to go through oauth2-proxy
	// since direct access to the protected port is blocked by iptables
	if effectiveCfg.BlockDirectAccess {
		rewrites, err := rewriteProbesForBlockedAccess(pod, effectiveCfg.ProtectedPort, mapping)
		if err != nil {
			return nil, err
		}
		for _, rw := range rewrites {
			patchBuilder.ReplaceProbePort(rw.ContainerIndex, rw.ProbeType, rw.HandlerType, rw.NewPort)
		}
	}

	for _, v := range volumes {
		patchBuilder.AddVolume(v)
	}

	// Handle Knative: redirect queue-proxy's USER_PORT to oauth2-proxy
	if err := m.patchKnativeQueueProxy(pod, patchBuilder); err != nil {
		return nil, err
	}

	return patchBuilder.AddAnnotation(InjectedAnnotation, "true").Build(), nil
}

// patchKnativeQueueProxy patches queue-proxy's USER_PORT env var to point to oauth2-proxy
// This is a no-op for non-Knative pods
func (m *PodMutator) patchKnativeQueueProxy(pod *corev1.Pod, patchBuilder *JSONPatchBuilder) error {
	if !m.knativeDetector.IsKnativePod(pod) {
		return nil
	}
	c, b := m.knativeDetector.FindQueueProxyIndex(pod)
	if !b {
		return fmt.Errorf("unexpected state: queue-proxy pod not found")
	}
	i := FindUserPortEnvIndex(pod, c)
	if i == -1 {
		return fmt.Errorf("unexpected state: USER_PORT env not found")
	}
	patchBuilder.ReplaceEnvVarValue(c, i, "4180")

	return nil
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

// rewriteProbePortNames finds all probes that reference the protected port name
// and returns rewrite descriptors for them.
//
// When oauth2-proxy takes over a named port (e.g., "http"), any probes in the
// original container that reference that port by name will break. This function
// identifies those probes so they can be patched to use the numeric port instead.
func rewriteProbePortNames(pod *corev1.Pod, protectedPortName string, originalPortNumber int32) []probeRewrite {
	ret := []probeRewrite{}

	for i, c := range pod.Spec.Containers {
		if v := checkProbe(c.LivenessProbe, "livenessProbe", i, protectedPortName, originalPortNumber); v != nil {
			ret = append(ret, *v)
		}
		if v := checkProbe(c.ReadinessProbe, "readinessProbe", i, protectedPortName, originalPortNumber); v != nil {
			ret = append(ret, *v)
		}
		if v := checkProbe(c.StartupProbe, "startupProbe", i, protectedPortName, originalPortNumber); v != nil {
			ret = append(ret, *v)
		}
	}

	return ret
}

// checkProbe checks if a probe references the protected port name and needs rewriting
func checkProbe(probe *corev1.Probe, probeType string, containerIndex int, protectedPortName string, originalPortNumber int32) *probeRewrite {
	if probe == nil {
		return nil
	}
	if probe.HTTPGet != nil && probe.HTTPGet.Port.Type == intstr.String && probe.HTTPGet.Port.StrVal == protectedPortName {
		return &probeRewrite{
			ContainerIndex: containerIndex,
			ProbeType:      probeType,
			HandlerType:    "httpGet",
			NewPort:        originalPortNumber,
		}
	}
	if probe.TCPSocket != nil && probe.TCPSocket.Port.Type == intstr.String && probe.TCPSocket.Port.StrVal == protectedPortName {
		return &probeRewrite{
			ContainerIndex: containerIndex,
			ProbeType:      probeType,
			HandlerType:    "tcpSocket",
			NewPort:        originalPortNumber,
		}
	}
	return nil
}

// probeRewrite describes a probe port that needs to be rewritten from name to number
type probeRewrite struct {
	ContainerIndex int
	ProbeType      string // "livenessProbe", "readinessProbe", "startupProbe"
	HandlerType    string // "httpGet" or "tcpSocket"
	NewPort        int32
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

// rewriteProbesForBlockedAccess finds all probes that target the protected port
// and returns rewrite descriptors to redirect them through oauth2-proxy.
//
// When block-direct-access is enabled, iptables blocks direct access to the protected port.
// Kubelet health checks come from the node (not localhost), so they'll be blocked.
// This function rewrites them to use the oauth2-proxy port instead.
func rewriteProbesForBlockedAccess(pod *corev1.Pod, protectedPort string, mapping PortMapping) ([]probeRewrite, error) {
	var ret []probeRewrite
	var port int
	var err error
	
	if annotation.IsNamedPort(protectedPort) {
		port = int(mapping.ProxyPort)
	} else {
		port, err = strconv.Atoi(protectedPort)
		if err != nil {
			return nil, err
		}
	}
	for i, c := range pod.Spec.Containers {
		if rw := checkProbeForBlockedAccess(c.LivenessProbe, "livenessProbe", i, protectedPort, int32(port), 4180); rw != nil {
			ret = append(ret, *rw)
		}
		if rw := checkProbeForBlockedAccess(c.ReadinessProbe, "readinessProbe", i, protectedPort, int32(port), 4180); rw != nil {
			ret = append(ret, *rw)
		}
		if rw := checkProbeForBlockedAccess(c.StartupProbe, "startupProbe", i, protectedPort, int32(port), 4180); rw != nil {
			ret = append(ret, *rw)
		}
	}

	return ret, nil
}

// checkProbeForBlockedAccess checks if a probe targets the protected port
// and needs rewriting for blocked access mode
func checkProbeForBlockedAccess(probe *corev1.Probe, probeType string, containerIndex int, protectedPortName string, protectedPortNumber int32, oauth2ProxyPort int32) *probeRewrite {
	if probe == nil {
		return nil
	}
	var handlerType string
	var port *intstr.IntOrString
	if probe.HTTPGet != nil {
		handlerType = "httpGet"
		port = &probe.HTTPGet.Port
	} else if probe.TCPSocket != nil {
		handlerType = "tcpSocket"
		port = &probe.TCPSocket.Port
	}
	if port == nil {
		return nil
	}
	
	if (port.Type == intstr.String && port.StrVal == protectedPortName) || (port.Type == intstr.Int && port.IntVal == protectedPortNumber) {
		return &probeRewrite{
			ContainerIndex: containerIndex,
			ProbeType: probeType,
			HandlerType: handlerType,
			NewPort: oauth2ProxyPort,
		}
	}
	return nil
}
