package mutation

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/spacemule/oauth2-proxy-injector/internal/annotation"
	"github.com/spacemule/oauth2-proxy-injector/internal/config"
)

// PortMapping represents the mapping between proxy and upstream ports
type PortMapping struct {
	// ProxyPort is the port oauth2-proxy forwards to (the app's original port)
	ProxyPort int32

	// TLSMode sets if the upstream is http, https, or https without TLS validation
	TLSMode annotation.UpstreamTLSMode
}

// SidecarBuilder defines the interface for building oauth2-proxy sidecar containers
type SidecarBuilder interface {
	// Build creates an oauth2-proxy container configured for the given port and settings
	Build(cfg *config.EffectiveConfig, portMapping PortMapping) (*corev1.Container, []corev1.Volume)
}

// OAuth2ProxySidecarBuilder implements SidecarBuilder for oauth2-proxy
type OAuth2ProxySidecarBuilder struct{}

// NewSidecarBuilder creates a new OAuth2ProxySidecarBuilder
func NewSidecarBuilder() *OAuth2ProxySidecarBuilder {
	return &OAuth2ProxySidecarBuilder{}
}

// Build creates an oauth2-proxy sidecar container and associated volumes
func (b *OAuth2ProxySidecarBuilder) Build(cfg *config.EffectiveConfig, portMapping PortMapping) (*corev1.Container, []corev1.Volume) {
	portName := cfg.ProtectedPort
	if !annotation.IsNamedPort(portName) {
		portName = "oauth2-proxy"
	}

	ping := "/ping"
	ready := "/ready"
	if cfg.PingPath != "" {
		ping = cfg.PingPath
	}
	if cfg.ReadyPath != "" {
		ready = cfg.ReadyPath
	}

	container := &corev1.Container{
		Name:  "oauth2-proxy",
		Image: cfg.ProxyImage,
		Args:  buildArgs(cfg, portMapping),
		Env:   buildEnvVars(cfg),
		Ports: []corev1.ContainerPort{
			{
				Name:          portName,
				ContainerPort: 4180,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		LivenessProbe:  buildProbe(4180, ping),
		ReadinessProbe: buildProbe(4180, ready),
	}

	if cfg.ProxyResources != nil {
		container.Resources = *cfg.ProxyResources
	}

	volumes := []corev1.Volume{}

	return container, volumes
}

// buildArgs constructs the command-line arguments for oauth2-proxy
//
// Boolean flags use explicit --flag="true" or --flag="false" format for clarity
// and predictability. This avoids ambiguity around default values.
func buildArgs(cfg *config.EffectiveConfig, portMapping PortMapping) []string {
	var ret []string

	ret = append(ret, "--provider="+cfg.Provider)
	ret = append(ret, "--oidc-issuer-url="+cfg.OIDCIssuerURL)
	ret = append(ret, "--client-id="+cfg.ClientID)
	ret = append(ret, "--http-address=0.0.0.0:4180")

	if cfg.Upstream == "" {
		switch cfg.UpstreamTLS {
		case annotation.UpstreamNoTLS:
			ret = append(ret, fmt.Sprintf("--upstream=http://127.0.0.1:%d", portMapping.ProxyPort))
		case annotation.UpstreamTLSSecure:
			ret = append(ret, fmt.Sprintf("--upstream=https://127.0.0.1:%d", portMapping.ProxyPort))
		case annotation.UpstreamTLSInsecure:
			ret = append(ret, fmt.Sprintf("--upstream=https://127.0.0.1:%d", portMapping.ProxyPort))
			ret = append(ret, "--ssl-upstream-insecure-skip-verify=true")
		}
	} else {
		ret = append(ret, fmt.Sprintf("--upstream=%s", cfg.Upstream))
		if cfg.UpstreamTLS == annotation.UpstreamTLSInsecure {
			ret = append(ret, "--ssl-upstream-insecure-skip-verify=true")
		}
	}

	if !cfg.CookieSecure {
		ret = append(ret, "--cookie-secure=false") // default is true
	}
	if cfg.SkipProviderButton {
		ret = append(ret, "--skip-provider-button=true") // default is false
	}
	if cfg.SkipJWTBearerTokens {
		ret = append(ret, "--skip-jwt-bearer-tokens=true") // default is false
	}
	if cfg.PassAccessToken {
		ret = append(ret, "--pass-access-token=true") // default is false
	}
	if cfg.SetXAuthRequest {
		ret = append(ret, "--set-xauthrequest=true") // default is false
	}
	if cfg.PassAuthorizationHeader {
		ret = append(ret, "--pass-authorization-header=true") // default is false
	}

	if cfg.PKCEEnabled {
		ret = append(ret, "--code-challenge-method=S256")
		ret = append(ret, "--client-secret-file=/dev/null")
	}

	if cfg.Scope != "" {
		ret = append(ret, "--scope="+cfg.Scope)
	}
	if cfg.OIDCGroupsClaim != "" {
		ret = append(ret, "--oidc-groups-claim="+cfg.OIDCGroupsClaim)
	}
	if cfg.RedirectURL != "" {
		ret = append(ret, "--redirect-url="+cfg.RedirectURL)
	}
	if cfg.CookieName != "" {
		ret = append(ret, "--cookie-name="+cfg.CookieName)
	}
	if cfg.PingPath != "" {
		ret = append(ret, "--ping-path="+cfg.PingPath)
	}
	if cfg.ReadyPath != "" {
		ret = append(ret, "--ready-path="+cfg.ReadyPath)
	}

	if len(cfg.ExtraJWTIssuers) > 0 {
		ret = append(ret, "--extra-jwt-issuers="+strings.Join(cfg.ExtraJWTIssuers, ","))
	}

	for _, d := range cfg.EmailDomains {
		ret = append(ret, "--email-domain="+d)
	}
	for _, g := range cfg.AllowedGroups {
		ret = append(ret, "--allowed-group="+g)
	}
	for _, p := range cfg.IgnorePaths {
		ret = append(ret, "--skip-auth-route="+p)
	}
	for _, p := range cfg.APIPaths {
		ret = append(ret, "--api-route="+p)
	}
	// for _, d := range cfg.AllowedEmails {
	// 	ret = append(ret, "--email-domain=" + d)
	// }
	for _, p := range cfg.CookieDomains {
		ret = append(ret, "--cookie-domain="+p)
	}
	for _, p := range cfg.WhitelistDomains {
		ret = append(ret, "--whitelist-domain="+p)
	}
	for _, arg := range cfg.ExtraArgs {
		ret = append(ret, arg)
	}

	return ret
}

// buildEnvVars creates environment variable definitions for secrets
func buildEnvVars(cfg *config.EffectiveConfig) []corev1.EnvVar {
	ret := []corev1.EnvVar{
		corev1.EnvVar{
			Name: "OAUTH2_PROXY_COOKIE_SECRET",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cfg.CookieSecretRef.Name,
					},
					Key: cfg.CookieSecretRef.Key,
				},
			},
		},
	}
	if cfg.ClientSecretRef != nil {
		ret = append(ret, corev1.EnvVar{
			Name: "OAUTH2_PROXY_CLIENT_SECRET",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cfg.ClientSecretRef.Name,
					},
					Key: cfg.ClientSecretRef.Key,
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
		PeriodSeconds:       10,
		TimeoutSeconds:      2,
	}
}

// CalculatePortMapping determines proxy->upstream port mapping
func CalculatePortMapping(
	containerPorts []corev1.ContainerPort,
	cfg *config.EffectiveConfig,
) (PortMapping, error) {
	if annotation.IsNamedPort(cfg.ProtectedPort) {
		for _, p := range containerPorts {
			if p.Name == cfg.ProtectedPort {
				return PortMapping{
					ProxyPort: p.ContainerPort,
					TLSMode:   cfg.UpstreamTLS,
				}, nil
			}
		}
	} else {
		portNum, err := strconv.Atoi(cfg.ProtectedPort)
		if err != nil {
			return PortMapping{}, err
		}
		for _, p := range containerPorts {
			if p.ContainerPort == int32(portNum) {
				return PortMapping{
					ProxyPort: p.ContainerPort,
					TLSMode:   cfg.UpstreamTLS,
				}, nil
			}
		}
	}
	return PortMapping{}, fmt.Errorf("matching port name %s not found", cfg.ProtectedPort)
}

// appendBoolFlag appends a boolean flag in --flag=true or --flag=false format
func appendBoolFlag(args []string, flagName string, value bool) []string {
	s := fmt.Sprintf("%s=%t", flagName, value)
	return append(args, s)
}
