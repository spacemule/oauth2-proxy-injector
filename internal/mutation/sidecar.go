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
//
// When cfg.SecretProviderClass is set:
//   - Adds CSI volume for the SecretProviderClass
//   - Adds volume mount to the container
//   - Uses file-based secret args (--client-secret-file, --cookie-secret-file)
//   - Skips env vars for secrets (they come from files instead)
//
// TODO:
// 1. Check if cfg.SecretProviderClass is set
// 2. If set:
//    a. Call BuildCSIVolume(cfg.SecretProviderClass) to create the CSI volume
//    b. Append the volume to the volumes slice
//    c. Call BuildCSIVolumeMount() to create the volume mount
//    d. Call buildArgsWithSecretFiles(cfg, portMapping) instead of buildArgs
//    e. Set container.Env to empty slice (secrets come from files)
//    f. Add the volume mount to container.VolumeMounts
// 3. If not set:
//    a. Use existing buildArgs(cfg, portMapping)
//    b. Use existing buildEnvVars(cfg)
// 4. Return container and volumes
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
//
// SourcedValue handling:
//   - IsLiteral(): add --flag=<value> as normal
//   - IsFromEnv(): skip the flag entirely (oauth2-proxy reads OAUTH2_PROXY_* env vars)
//   - IsFromFile(): use --*-file flag pointing to CSI mount (only for secrets)
//
// TODO: Update each flag to check its source type before adding:
// 1. For provider: if IsFromEnv(), skip --provider flag
// 2. For oidc-issuer-url: if IsFromEnv(), skip flag
// 3. For client-id: if IsFromEnv(), skip flag
// 4. For scope: if IsFromEnv(), skip flag
// 5. For validate-url: if IsFromEnv(), skip flag
// 6. For redirect-url: if IsFromEnv(), skip flag
// 7. For upstream: if IsFromEnv(), skip flag
// 8. For client-secret: if IsFromFile(), use --client-secret-file; if IsFromEnv(), skip
// 9. For cookie-secret: if IsFromFile(), use --cookie-secret-file; if IsFromEnv(), skip
func buildArgs(cfg *config.EffectiveConfig, portMapping PortMapping) []string {
	var ret []string

	// Provider - skip if fromEnv
	if !cfg.Provider.IsFromEnv() {
		ret = append(ret, "--provider="+cfg.Provider.Value)
	}

	// OIDC Issuer URL - skip if fromEnv
	if !cfg.OIDCIssuerURL.IsFromEnv() && cfg.OIDCIssuerURL.Value != "" {
		ret = append(ret, "--oidc-issuer-url="+cfg.OIDCIssuerURL.Value)
	}

	// Client ID - skip if fromEnv
	if !cfg.ClientID.IsFromEnv() {
		ret = append(ret, "--client-id="+cfg.ClientID.Value)
	}

	ret = append(ret, "--http-address=0.0.0.0:4180")

	// Upstream - skip entirely if fromEnv (oauth2-proxy reads OAUTH2_PROXY_UPSTREAM)
	if !cfg.Upstream.IsFromEnv() {
		if cfg.Upstream.Value == "" {
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
			ret = append(ret, fmt.Sprintf("--upstream=%s", cfg.Upstream.Value))
			if cfg.UpstreamTLS == annotation.UpstreamTLSInsecure {
				ret = append(ret, "--ssl-upstream-insecure-skip-verify=true")
			}
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

	// Handle PKCE / code-challenge-method
	// If CodeChallengeMethod is explicitly set, use it (regardless of PKCEEnabled)
	// If PKCEEnabled is true and CodeChallengeMethod is empty, default to S256
	if cfg.CodeChallengeMethod != "" {
		ret = append(ret, "--code-challenge-method="+cfg.CodeChallengeMethod)
		// Only need /dev/null if no client secret is being provided by any source
		needsNullSecret := cfg.ClientSecret.Ref == nil && cfg.ClientSecret.IsLiteral()
		if needsNullSecret {
			ret = append(ret, "--client-secret-file=/dev/null")
		}
	} else if cfg.PKCEEnabled {
		ret = append(ret, "--code-challenge-method=S256")
		// Only need /dev/null if no client secret is being provided by any source
		needsNullSecret := cfg.ClientSecret.Ref == nil && cfg.ClientSecret.IsLiteral()
		if needsNullSecret {
			ret = append(ret, "--client-secret-file=/dev/null")
		}
	}

	// Handle file-based secrets (from CSI SecretProviderClass)
	// When source is file, add --*-secret-file flags pointing to CSI mount
	if cfg.ClientSecret.IsFromFile() {
		ret = append(ret, "--client-secret-file="+GetFileOverridePath("client-secret"))
	}
	if cfg.CookieSecret.IsFromFile() {
		ret = append(ret, "--cookie-secret-file="+GetFileOverridePath("cookie-secret"))
	}

	// Scope - skip if fromEnv
	if !cfg.Scope.IsFromEnv() && cfg.Scope.Value != "" {
		ret = append(ret, "--scope="+cfg.Scope.Value)
	}
	// Validate URL - skip if fromEnv
	if !cfg.ValidateURL.IsFromEnv() && cfg.ValidateURL.Value != "" {
		ret = append(ret, "--validate-url="+cfg.ValidateURL.Value)
	}
	if cfg.OIDCGroupsClaim != "" {
		ret = append(ret, "--oidc-groups-claim="+cfg.OIDCGroupsClaim)
	}
	// Redirect URL - skip if fromEnv
	if !cfg.RedirectURL.IsFromEnv() && cfg.RedirectURL.Value != "" {
		ret = append(ret, "--redirect-url="+cfg.RedirectURL.Value)
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

// buildArgsWithSecretFiles constructs oauth2-proxy arguments using file-based secrets
// from the SecretProviderClass CSI mount instead of environment variables.
//
// DEPRECATED: This function is no longer needed since buildArgs now handles
// ValueSourceFile directly via the ClientSecretSource and CookieSecretSource fields.
// Keeping for backwards compatibility with SecretProviderClass-based workflows.
//
// This is used when cfg.SecretProviderClass is set. Secrets are read from files
// at SecretProviderMountPath instead of from Kubernetes Secrets via env vars.
//
// TODO:
// 1. Call buildArgs(cfg, portMapping) to get the base args
// 2. The file-based secret flags are now added by buildArgs when source is ValueSourceFile
// 3. Return the args slice
func buildArgsWithSecretFiles(cfg *config.EffectiveConfig, portMapping PortMapping) []string {
	return buildArgs(cfg, portMapping)
}

// buildEnvVars creates environment variable definitions for secrets
//
// SourcedSecretRef handling:
//   - IsLiteral(): create env var from Ref (current behavior)
//   - IsFromFile(): skip env var (secret read from file via --*-secret-file)
//   - IsFromEnv(): skip env var (oauth2-proxy reads from pre-existing env var)
//
// TODO:
// 1. For cookie-secret:
//    a. If IsLiteral() and Ref != nil:
//       - Add OAUTH2_PROXY_COOKIE_SECRET env var from SecretRef
//    b. If IsFromFile() or IsFromEnv():
//       - Skip (handled by --cookie-secret-file or existing env var)
// 2. For client-secret:
//    a. If IsLiteral() and Ref != nil:
//       - Add OAUTH2_PROXY_CLIENT_SECRET env var from SecretRef
//    b. If IsFromFile() or IsFromEnv():
//       - Skip (handled by --client-secret-file or existing env var)
func buildEnvVars(cfg *config.EffectiveConfig) []corev1.EnvVar {
	ret := []corev1.EnvVar{}

	// Cookie secret - only add env var if source is literal and ref is set
	if cfg.CookieSecret.IsLiteral() && cfg.CookieSecret.Ref != nil {
		ret = append(ret, corev1.EnvVar{
			Name: "OAUTH2_PROXY_COOKIE_SECRET",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cfg.CookieSecret.Ref.Name,
					},
					Key: cfg.CookieSecret.Ref.Key,
				},
			},
		})
	}

	// Client secret - only add env var if source is literal and ref is set
	if cfg.ClientSecret.IsLiteral() && cfg.ClientSecret.Ref != nil {
		ret = append(ret, corev1.EnvVar{
			Name: "OAUTH2_PROXY_CLIENT_SECRET",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cfg.ClientSecret.Ref.Name,
					},
					Key: cfg.ClientSecret.Ref.Key,
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
