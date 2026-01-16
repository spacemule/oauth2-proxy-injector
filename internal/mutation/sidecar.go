package mutation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/spacemule/oauth2-proxy-injector/internal/annotation"
	"github.com/spacemule/oauth2-proxy-injector/internal/config"
)

// PortMapping represents the mapping between proxy and upstream ports
type PortMapping struct {
	// ProxyPort is the port oauth2-proxy proxies (internal-facing)
	ProxyPort int32

	// HostPort is the port oauth2-proxy listens on the host IP (external-facing)
	HostPort int32

	// TLSMode sets if the upstream is http, https, or https without TLS validation
	TLSMode annotation.UpstreamTLSMode
}

// SidecarBuilder defines the interface for building oauth2-proxy sidecar containers
type SidecarBuilder interface {
	// Build creates an oauth2-proxy container configured for the given port and settings
	Build(proxyCfg *config.ProxyConfig, portMapping PortMapping, annotationCfg *annotation.Config) (*corev1.Container, []corev1.Volume)
}

// OAuth2ProxySidecarBuilder implements SidecarBuilder for oauth2-proxy
type OAuth2ProxySidecarBuilder struct{}

// NewSidecarBuilder creates a new OAuth2ProxySidecarBuilder
func NewSidecarBuilder() *OAuth2ProxySidecarBuilder {
	return &OAuth2ProxySidecarBuilder{}
}

// Build creates an oauth2-proxy sidecar container and associated volumes
func (b *OAuth2ProxySidecarBuilder) Build(
	proxyCfg *config.ProxyConfig,
	portMapping PortMapping,
	annotationCfg *annotation.Config,
) (*corev1.Container, []corev1.Volume) {
	container := &corev1.Container{
		Name: "oauth2-proxy",
		Image: proxyCfg.ProxyImage,
		Args: buildArgs(proxyCfg, portMapping, annotationCfg),
		Env: buildEnvVars(proxyCfg),
		Ports: []corev1.ContainerPort{
			corev1.ContainerPort{
				Name: annotationCfg.ProtectedPort,
				ContainerPort: 4180,
				HostPort: portMapping.HostPort,
				Protocol: corev1.ProtocolTCP,
			},
		},
		LivenessProbe: buildProbe(4180, "/ping"),
		ReadinessProbe: buildProbe(4180, "/ready"),
	}

	if proxyCfg.ProxyResources != nil {
		container.Resources = *proxyCfg.ProxyResources
	}
	
	volumes := []corev1.Volume{}

	return container, volumes
}

func buildArgs(proxyCfg *config.ProxyConfig, portMapping PortMapping, annotationCfg *annotation.Config) []string {
	var ret []string
	
	ret = append(ret, "--provider=" + proxyCfg.Provider)
	ret = append(ret, "--oidc-issuer-url=" + proxyCfg.OIDCIssuerURL)
	ret = append(ret, "--client-id=" + proxyCfg.ClientID)
	ret = append(ret, "--http-address=0.0.0.0:4180" )
	
	switch annotationCfg.UpstreamTLS {
	case annotation.UpstreamNoTLS:
		ret = append(ret, fmt.Sprintf("--upstream=http://127.0.0.1:%d", portMapping.ProxyPort))
	case annotation.UpstreamTLSSecure:
		ret = append(ret, fmt.Sprintf("--upstream=https://127.0.0.1:%d", portMapping.ProxyPort))
	case annotation.UpstreamTLSInsecure:
		ret = append(ret, fmt.Sprintf("--upstream=https://127.0.0.1:%d", portMapping.ProxyPort))
		ret = append(ret, "--ssl-upstream-insecure-skip-verify")
	}
	
	if proxyCfg.SkipProviderButton {
		ret = append(ret, "--skip-provider-button")
	}
	if proxyCfg.PKCEEnabled {
		ret = append(ret, "--code-challenge-method=S256")
		ret = append(ret, "--client-secret-file=/dev/null")
	}
	if annotationCfg.SkipJWTBearerTokens {
		ret = append(ret, "--skip-jwt-bearer-tokens")
	}

	for _, d := range proxyCfg.EmailDomains {
		ret = append(ret, "--email-domain=" + d)
	}
	for _, g := range proxyCfg.AllowedGroups {
		ret = append(ret, "--allowed-group=" + g)
	}
	for _, p := range annotationCfg.IgnorePaths {
		ret = append(ret, "--skip-auth-route=" + p)
	}
	for _, p := range annotationCfg.APIPaths {
		ret = append(ret, "--api-route=" + p)
	}
	for _, arg := range proxyCfg.ExtraArgs {
		ret = append(ret, arg)
	}

	return ret
}

// buildEnvVars creates environment variable definitions for secrets
func buildEnvVars(proxyCfg *config.ProxyConfig) []corev1.EnvVar {
	ret := []corev1.EnvVar{
		corev1.EnvVar{
			Name: "OAUTH2_PROXY_COOKIE_SECRET",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: proxyCfg.CookieSecretRef.Name,
					},
					Key: proxyCfg.CookieSecretRef.Key,
				},
			},
		},
	}
	if proxyCfg.ClientSecretRef != nil {
		ret = append(ret, corev1.EnvVar{
			Name: "OAUTH2_PROXY_CLIENT_SECRET",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: proxyCfg.ClientSecretRef.Name,
					},
					Key: proxyCfg.ClientSecretRef.Key,
				},
			},
		})
	}

	return ret
}

// buildProbe creates a liveness/readiness probe for oauth2-proxy
func buildProbe(port int32, path string) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt32(port),
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds: 10,
		TimeoutSeconds: 2,
	}
}

// CalculatePortMappings determines proxy->upstream port mapping
func CalculatePortMapping(
	containerPorts []corev1.ContainerPort,
	annotationCfg *annotation.Config,
) (PortMapping, error) {
	for _, p := range containerPorts {
		if p.Name == annotationCfg.ProtectedPort {
			return PortMapping{
				ProxyPort: p.ContainerPort,
				HostPort: p.HostPort,
				TLSMode: annotationCfg.UpstreamTLS,
			}, nil
		}
	}
	return PortMapping{}, fmt.Errorf("matching port name %s not found", annotationCfg.ProtectedPort)
}