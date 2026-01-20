package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/spacemule/oauth2-proxy-injector/internal/annotation"
	"github.com/spacemule/oauth2-proxy-injector/internal/mutation"
)

// Annotation keys for Service mutation
const (
	// AnnotationPrefix is the base prefix for all oauth2-proxy annotations
	AnnotationPrefix = "spacemule.net/oauth2-proxy."

	// KeyRewritePorts specifies which Service ports should be routed through oauth2-proxy
	// Value: comma-separated port names or numbers (e.g., "http,https" or "8080,8443")
	// Only these ports will have their targetPort rewritten
	KeyRewritePorts = AnnotationPrefix + "rewrite-ports"

	// KeyProxyPort specifies the port oauth2-proxy listens on in the pod
	// Value: port number (default: "4180")
	// This is what targetPort gets rewritten to
	KeyProxyPort = AnnotationPrefix + "proxy-port"

	// KeyInjected is set by the webhook after mutation to prevent double-mutation
	// Value: "true"
	KeyInjected = AnnotationPrefix + "service-injected"

	// OriginalTargetPortPrefix is used to store original targetPort values
	// Format: spacemule.net/oauth2-proxy.original-target.<port-name-or-number>=<original-value>
	OriginalTargetPortPrefix = AnnotationPrefix + "original-target."
)

// DefaultProxyPort is the default port oauth2-proxy listens on
const DefaultProxyPort = 4180

// Mutator defines the contract for Service mutation operations
type Mutator interface {
	// Mutate takes a Service and returns JSON patch operations to rewrite ports
	Mutate(ctx context.Context, svc *corev1.Service) ([]mutation.PatchOperation, error)
}

// ServiceMutator implements Mutator for oauth2-proxy port rewriting
type ServiceMutator struct{}

// ServicePatchBuilder builds JSON patches for Service mutations
// Simpler than the full PatchBuilder since Services only need:
// - ReplaceTargetPort
// - AddAnnotation
type ServicePatchBuilder struct {
	operations     []mutation.PatchOperation
	hasAnnotations bool
}

// NewServicePatchBuilder creates a new builder for Service patches
func NewServicePatchBuilder(hasAnnotations bool) *ServicePatchBuilder {
	return &ServicePatchBuilder{
		operations:     []mutation.PatchOperation{},
		hasAnnotations: hasAnnotations,
	}
}

// ReplaceTargetPort replaces the targetPort at the given port index
func (b *ServicePatchBuilder) ReplaceTargetPort(portIndex int, newPort int32) *ServicePatchBuilder {
	b.operations = append(b.operations, mutation.PatchOperation{
		Op:    "add", //add overwrites. so this is safe if the targetport is missing
		Path:  fmt.Sprintf("/spec/ports/%d/targetPort", portIndex),
		Value: newPort,
	})
	return b
}

// AddAnnotation adds or updates an annotation on the Service
func (b *ServicePatchBuilder) AddAnnotation(key, value string) *ServicePatchBuilder {
	if !b.hasAnnotations {
		b.operations = append(b.operations, mutation.PatchOperation{
			Op:    "add",
			Path:  "/metadata/annotations",
			Value: map[string]string{},
		})

		b.hasAnnotations = true
	}
	b.operations = append(b.operations, mutation.PatchOperation{
		Op:    "add",
		Path:  "/metadata/annotations/" + escapeJSONPointer(key),
		Value: value,
	})

	return b
}

// Build returns the accumulated patch operations
func (b *ServicePatchBuilder) Build() []mutation.PatchOperation {
	ret := make([]mutation.PatchOperation, len(b.operations))
	copy(ret, b.operations)
	return ret
}

// escapeJSONPointer escapes special characters per RFC 6901
func escapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	return strings.ReplaceAll(s, "/", "~1")
}

// NewServiceMutator creates a new ServiceMutator
func NewServiceMutator() *ServiceMutator {
	return &ServiceMutator{}
}

// Mutate inspects Service annotations and rewrites targetPort for specified ports
//
// TODO: Implement this function
//  1. Check if KeyInjected annotation exists - if so, return empty patch (already mutated)
//  2. Check if KeyRewritePorts annotation exists - if not, return empty patch (not opted in)
//  3. Parse KeyRewritePorts into a list of port identifiers (names or numbers)
//  4. Parse KeyProxyPort if set, otherwise use DefaultProxyPort
//  5. For each port in the Service spec:
//     a. Check if it matches any identifier in the rewrite list
//     b. If yes:
//     - Store original targetPort in annotation (OriginalTargetPortPrefix + portName)
//     - Rewrite targetPort to proxy port
//  6. Add KeyInjected annotation
//  7. Return patch operations
func (m *ServiceMutator) Mutate(ctx context.Context, svc *corev1.Service) ([]mutation.PatchOperation, error) {
	if isAlreadyInjected(svc) {
		return nil, nil
	}

	cfg, err := ParseServiceAnnotations(svc.Annotations)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, nil
	}

	return buildServicePatches(svc, cfg)
}

// ServiceConfig holds parsed annotation values for a Service
type ServiceConfig struct {
	// RewritePorts is the list of port names or numbers to rewrite
	RewritePorts []string

	// ProxyPort is the port oauth2-proxy listens on (default: 4180)
	ProxyPort int32
}

// ParseServiceAnnotations extracts oauth2-proxy configuration from Service annotations
func ParseServiceAnnotations(annotations map[string]string) (*ServiceConfig, error) {
	v, ok := annotations[KeyRewritePorts]
	if !ok {
		return nil, nil
	}
	ret := &ServiceConfig{
		RewritePorts: strings.Split(strings.TrimSpace(v), ","),
	}
	if p, ok := annotations[KeyProxyPort]; ok {
		intPort, err := strconv.ParseInt(p, 10, 32)
		if err != nil {
			return nil, err
		}
		if intPort < 1 || intPort > 65535 {
			return nil, fmt.Errorf("%s value: %d not in valid port range", KeyProxyPort, intPort)
		}
		ret.ProxyPort = int32(intPort)
	} else {
		ret.ProxyPort = 4180
	}

	return ret, nil

}

// shouldRewritePort checks if a ServicePort should have its targetPort rewritten
func shouldRewritePort(port corev1.ServicePort, rewritePorts []string) (bool, error) {
	for _, p := range rewritePorts {
		if annotation.IsNamedPort(p) {
			if port.Name == p {
				return true, nil
			}
		} else {
			intPort, err := strconv.ParseInt(p, 10, 32)
			if err != nil {
				return false, err
			}
			if port.TargetPort == intstr.FromInt32(int32(intPort)) || port.Port == int32(intPort) {
				return true, nil
			}
		}
	}

	return false, nil
}

// getPortIdentifier returns a stable identifier for a ServicePort
// Used as the suffix for OriginalTargetPortPrefix annotation
func getPortIdentifier(port corev1.ServicePort) string {
	if port.Name != "" {
		return port.Name
	}
	return fmt.Sprintf("%d", port.Port)
}

// buildServicePatches creates JSON patch operations for rewriting Service ports
func buildServicePatches(svc *corev1.Service, cfg *ServiceConfig) ([]mutation.PatchOperation, error) {
	builder := NewServicePatchBuilder(hasExistingAnnotations(svc))
	for i, p := range svc.Spec.Ports {
		b, err := shouldRewritePort(p, cfg.RewritePorts)
		if err != nil {
			return nil, err
		}
		if b {
			portId := getPortIdentifier(p)
			switch p.TargetPort.Type {
			case intstr.Int:
				builder.AddAnnotation(OriginalTargetPortPrefix+portId, strconv.Itoa(int(p.TargetPort.IntVal)))
			case intstr.String:
				builder.AddAnnotation(OriginalTargetPortPrefix+portId, p.TargetPort.StrVal)
			default:
				builder.AddAnnotation(OriginalTargetPortPrefix+portId, strconv.Itoa(int(p.Port)))
			}
			builder.ReplaceTargetPort(i, cfg.ProxyPort)
		}
	}
	builder.AddAnnotation(KeyInjected, "true")
	return builder.Build(), nil
}

// isAlreadyInjected checks if the Service has already been mutated
func isAlreadyInjected(svc *corev1.Service) bool {
	_, ok := svc.Annotations[KeyInjected]
	return ok
}

// hasExistingAnnotations checks if the Service has any annotations
func hasExistingAnnotations(svc *corev1.Service) bool {
	return len(svc.Annotations) > 0
}
