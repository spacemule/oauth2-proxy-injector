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
//   - File-based secrets are handled by buildArgs via IsFromFile() checks
//   - Env vars for secrets are skipped by buildEnvVars via IsFromFile() checks
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

	// Add CSI volume and mount when SecretProviderClass is configured
	if cfg.SecretProviderClass != "" {
		volumes = append(volumes, BuildCSIVolume(cfg.SecretProviderClass))
		container.VolumeMounts = append(container.VolumeMounts, BuildCSIVolumeMount())
	}

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

	// Cookie secure - skip if fromEnv, otherwise only add if false (default is true)
	if !cfg.CookieSecure.IsFromEnv() && !cfg.CookieSecure.Value {
		ret = append(ret, "--cookie-secure=false")
	}
	// Skip provider button - skip if fromEnv, otherwise only add if true (default is false)
	if !cfg.SkipProviderButton.IsFromEnv() && cfg.SkipProviderButton.Value {
		ret = append(ret, "--skip-provider-button=true")
	}
	// Skip JWT bearer tokens - skip if fromEnv, otherwise only add if true (default is false)
	if !cfg.SkipJWTBearerTokens.IsFromEnv() && cfg.SkipJWTBearerTokens.Value {
		ret = append(ret, "--skip-jwt-bearer-tokens=true")
	}
	// Pass access token - skip if fromEnv, otherwise only add if true (default is false)
	if !cfg.PassAccessToken.IsFromEnv() && cfg.PassAccessToken.Value {
		ret = append(ret, "--pass-access-token=true")
	}
	// Set X-Auth-Request headers - skip if fromEnv, otherwise only add if true (default is false)
	if !cfg.SetXAuthRequest.IsFromEnv() && cfg.SetXAuthRequest.Value {
		ret = append(ret, "--set-xauthrequest=true")
	}
	// Pass authorization header - skip if fromEnv, otherwise only add if true (default is false)
	if !cfg.PassAuthorizationHeader.IsFromEnv() && cfg.PassAuthorizationHeader.Value {
		ret = append(ret, "--pass-authorization-header=true")
	}

	// Handle PKCE / code-challenge-method
	// If CodeChallengeMethod is explicitly set, use it (regardless of PKCEEnabled)
	// If PKCEEnabled is true and CodeChallengeMethod is empty, default to S256
	// Skip CodeChallengeMethod if fromEnv (oauth2-proxy reads from env vars)
	// Note: PKCEEnabled is a bool abstraction that doesn't support fromEnv
	if !cfg.CodeChallengeMethod.IsFromEnv() && cfg.CodeChallengeMethod.Value != "" {
		ret = append(ret, "--code-challenge-method="+cfg.CodeChallengeMethod.Value)
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
	// OIDC groups claim - skip if fromEnv
	if !cfg.OIDCGroupsClaim.IsFromEnv() && cfg.OIDCGroupsClaim.Value != "" {
		ret = append(ret, "--oidc-groups-claim="+cfg.OIDCGroupsClaim.Value)
	}
	// Redirect URL - skip if fromEnv
	if !cfg.RedirectURL.IsFromEnv() && cfg.RedirectURL.Value != "" {
		ret = append(ret, "--redirect-url="+cfg.RedirectURL.Value)
	}
	// Cookie name - skip if fromEnv
	if !cfg.CookieName.IsFromEnv() && cfg.CookieName.Value != "" {
		ret = append(ret, "--cookie-name="+cfg.CookieName.Value)
	}
	if cfg.PingPath != "" {
		ret = append(ret, "--ping-path="+cfg.PingPath)
	}
	if cfg.ReadyPath != "" {
		ret = append(ret, "--ready-path="+cfg.ReadyPath)
	}

	// Extra JWT issuers - skip if fromEnv
	if !cfg.ExtraJWTIssuers.IsFromEnv() && len(cfg.ExtraJWTIssuers.Values) > 0 {
		ret = append(ret, "--extra-jwt-issuers="+strings.Join(cfg.ExtraJWTIssuers.Values, ","))
	}

	// Email domains - skip if fromEnv
	if !cfg.EmailDomains.IsFromEnv() {
		for _, d := range cfg.EmailDomains.Values {
			ret = append(ret, "--email-domain="+d)
		}
	}
	// Allowed groups - skip if fromEnv
	if !cfg.AllowedGroups.IsFromEnv() {
		for _, g := range cfg.AllowedGroups.Values {
			ret = append(ret, "--allowed-group="+g)
		}
	}
	// Ignore paths and API paths are annotation-only (no fromEnv support)
	for _, p := range cfg.IgnorePaths {
		ret = append(ret, "--skip-auth-route="+p)
	}
	for _, p := range cfg.APIPaths {
		ret = append(ret, "--api-route="+p)
	}
	// Cookie domains - skip if fromEnv
	if !cfg.CookieDomains.IsFromEnv() {
		for _, p := range cfg.CookieDomains.Values {
			ret = append(ret, "--cookie-domain="+p)
		}
	}
	// Whitelist domains - skip if fromEnv
	if !cfg.WhitelistDomains.IsFromEnv() {
		for _, p := range cfg.WhitelistDomains.Values {
			ret = append(ret, "--whitelist-domain="+p)
		}
	}
	for _, arg := range cfg.ExtraArgs {
		ret = append(ret, arg)
	}

	return ret
}

// buildEnvVars creates environment variable definitions for secrets and fromEnv fields
//
// SourcedSecretRef handling:
//   - IsLiteral(): create env var from Ref (current behavior)
//   - IsFromFile(): skip env var (secret read from file via --*-secret-file)
//   - IsFromEnv(): create env var from EnvSecret if set
//
// When EnvSecret is set and a field has IsFromEnv(), we generate:
//
//	env:
//	- name: OAUTH2_PROXY_CLIENT_ID
//	  valueFrom:
//	    secretKeyRef:
//	      name: <EnvSecret>
//	      key: client-id
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

	// If EnvSecret is set, generate env vars for all fromEnv fields
	if cfg.EnvSecret != "" {
		ret = append(ret, buildEnvVarsFromSecret(cfg)...)
	}

	return ret
}

// buildEnvVarsFromSecret generates env var entries for fields with fromEnv source
// Each entry maps the oauth2-proxy env var name to a key in the EnvSecret
func buildEnvVarsFromSecret(cfg *config.EffectiveConfig) []corev1.EnvVar {
	var ret []corev1.EnvVar

	secretName := cfg.EnvSecret

	// Helper to add an env var from the secret
	addEnvVar := func(envName, secretKey string) {
		ret = append(ret, corev1.EnvVar{
			Name: envName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: secretKey,
				},
			},
		})
	}

	// Provider settings
	if cfg.Provider.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_PROVIDER", "provider")
	}
	if cfg.OIDCIssuerURL.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_OIDC_ISSUER_URL", "oidc-issuer-url")
	}
	if cfg.OIDCGroupsClaim.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_OIDC_GROUPS_CLAIM", "oidc-groups-claim")
	}
	if cfg.Scope.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_SCOPE", "scope")
	}
	if cfg.ValidateURL.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_VALIDATE_URL", "validate-url")
	}

	// Identity settings
	if cfg.ClientID.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_CLIENT_ID", "client-id")
	}
	if cfg.ClientSecret.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_CLIENT_SECRET", "client-secret")
	}
	// code-challenge-method maps to OAUTH2_PROXY_CODE_CHALLENGE_METHOD
	// Note: pkce-enabled is a boolean abstraction (true → S256) and doesn't support fromEnv
	if cfg.CodeChallengeMethod.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_CODE_CHALLENGE_METHOD", "code-challenge-method")
	}

	// Cookie settings
	if cfg.CookieSecret.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_COOKIE_SECRET", "cookie-secret")
	}
	if cfg.CookieName.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_COOKIE_NAME", "cookie-name")
	}
	if cfg.CookieSecure.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_COOKIE_SECURE", "cookie-secure")
	}
	if cfg.CookieDomains.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_COOKIE_DOMAINS", "cookie-domains")
	}

	// Authorization settings
	if cfg.EmailDomains.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_EMAIL_DOMAINS", "email-domains")
	}
	if cfg.AllowedGroups.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_ALLOWED_GROUPS", "allowed-groups")
	}
	if cfg.WhitelistDomains.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_WHITELIST_DOMAINS", "whitelist-domains")
	}

	// Routing settings
	if cfg.RedirectURL.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_REDIRECT_URL", "redirect-url")
	}
	if cfg.ExtraJWTIssuers.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_EXTRA_JWT_ISSUERS", "extra-jwt-issuers")
	}
	if cfg.Upstream.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_UPSTREAMS", "upstream")
	}

	// Header settings
	if cfg.PassAccessToken.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_PASS_ACCESS_TOKEN", "pass-access-token")
	}
	if cfg.SetXAuthRequest.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_SET_XAUTHREQUEST", "set-xauthrequest")
	}
	if cfg.PassAuthorizationHeader.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_PASS_AUTHORIZATION_HEADER", "pass-authorization-header")
	}

	// Behavior settings
	if cfg.SkipProviderButton.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_SKIP_PROVIDER_BUTTON", "skip-provider-button")
	}
	if cfg.SkipJWTBearerTokens.IsFromEnv() {
		addEnvVar("OAUTH2_PROXY_SKIP_JWT_BEARER_TOKENS", "skip-jwt-bearer-tokens")
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
