package annotation

import (
	"fmt"
	"strconv"
	"strings"
)

// Annotation key constants - all under the spacemule.net domain
//
// Annotations are organized into categories:
// - Core: Enable/disable and ConfigMap reference
// - Port/Routing: How traffic flows through the proxy
// - Identity Overrides: Per-service OAuth app credentials
// - Authorization Overrides: Per-service access control
// - Routing Overrides: Per-service URLs and JWT settings
// - Header Overrides: Per-service token passing
// - Behavior Overrides: Per-service UX settings
const (
	// AnnotationPrefix is the base prefix for all oauth2-proxy annotations
	AnnotationPrefix = "spacemule.net/oauth2-proxy."

	// ===== Core Annotations =====

	// KeyEnabled indicates whether oauth2-proxy injection is enabled for this pod
	// Value: "true" or "false"
	KeyEnabled = AnnotationPrefix + "enabled"

	// KeyConfig optionally overrides the default ConfigMap name
	// Value: ConfigMap name (e.g., "plex-config")
	// If not set, uses the default ConfigMap configured in the webhook
	KeyConfig = AnnotationPrefix + "config"

	// KeyInjected is set by the webhook after injection to prevent double-injection
	// Value: "true" (set automatically, do not set manually)
	KeyInjected = AnnotationPrefix + "injected"

	// KeyBlockDirectAccess optionally disables direct access to the running service at the pod's IP
	// If enabled, an initContainer is added to the pod to run iptables and block access to the
	// protected container's protected port.
	// Value: "true" or "false" (default)
	KeyBlockDirectAccess = AnnotationPrefix + "block-direct-access"

	// ===== Port/Routing Annotations (annotation-only) =====

	// KeyProtectedPort specifies which container port should be protected
	// Value: port name (e.g., "http")
	// Default: "http"
	KeyProtectedPort = AnnotationPrefix + "protected-port"

	// KeyIgnorePaths specifies which paths should NOT be protected
	// Value: comma-separated regex
	// Format: method=path_regex OR method!=path_regex. For all methods: path_regex OR !=path_regex
	// Useful for metrics endpoints, swagger, health checks, etc.
	KeyIgnorePaths = AnnotationPrefix + "ignore-paths"

	// KeyAPIPaths specifies which paths should be protected by JWTs only
	// Value: comma-separated string of paths (e.g. "/api/,/v1")
	// Bypasses login on specified paths when JWT is provided.
	KeyAPIPaths = AnnotationPrefix + "api-paths"

	// KeySkipJWTBearerTokens specifies whether or not to skip login for requests with bearer tokens
	// Value: "true" or "false". Defaults to "false"
	// Bypasses login when valid JWT is provided.
	KeySkipJWTBearerTokens = AnnotationPrefix + "skip-jwt-bearer-tokens"

	// KeyUpstreamTLS specifies how to handle TLS when connecting to the upstream pod
	// Value: "http" (default), "https", or "https-insecure"
	KeyUpstreamTLS = AnnotationPrefix + "upstream-tls"

	// ===== Identity Overrides (override ConfigMap values) =====

	// KeyClientID overrides the OAuth2 client ID from ConfigMap
	// Use when this service has a different OAuth app registration
	KeyClientID = AnnotationPrefix + "client-id"

	// KeyClientSecretRef overrides the client secret reference from ConfigMap
	// Format: "secret-name" or "secret-name:key"
	KeyClientSecretRef = AnnotationPrefix + "client-secret-ref"

	// KeyCookieSecretRef overrides the cookie secret reference from ConfigMap
	// Format: "secret-name" or "secret-name:key"
	KeyCookieSecretRef = AnnotationPrefix + "cookie-secret-ref"

	// KeyScope overrides the scope from ConfigMap
	// Format: "scope0 scope1"
	KeyScope = AnnotationPrefix + "scope"

	// KeyValidateURL used to set validation URL for opaque tokens
	KeyValidateURL = AnnotationPrefix + "validate-url"

	// KeyPKCEEnabled overrides PKCE setting from ConfigMap
	// Value: "true" or "false"
	KeyPKCEEnabled = AnnotationPrefix + "pkce-enabled"

	// KeyCodeChallengeMethod overrides the PKCE code challenge method
	// Value: "S256" or "plain"
	// Note: If pkce-enabled is true and this is not set, S256 is used automatically
	// Only set this if you need to override the default or use PKCE without pkce-enabled
	KeyCodeChallengeMethod = AnnotationPrefix + "code-challenge-method"

	// ===== Authorization Overrides (override ConfigMap values) =====

	// KeyEmailDomains overrides allowed email domains from ConfigMap
	// Value: comma-separated domains (e.g., "example.com,corp.example.com")
	// Use "*" to allow all domains
	KeyEmailDomains = AnnotationPrefix + "email-domains"

	// KeyAllowedGroups overrides allowed groups from ConfigMap
	// Value: comma-separated group names
	KeyAllowedGroups = AnnotationPrefix + "allowed-groups"

	// KeyAllowedEmails overrides/adds allowed email addresses
	// Value: comma-separated email addresses
	// More granular than email-domains for sensitive services
	// KeyAllowedEmails = AnnotationPrefix + "allowed-emails"

	// KeyWhitelistDomains overrides/adds allowed domains
	// Value: comma-separated domains
	KeyWhitelistDomains = AnnotationPrefix + "whitelist-domains"

	// KeyName overrides the cookie name from ConfigMap
	// Format: "cookie-name
	KeyCookieName = AnnotationPrefix + "cookie-name"

	// KeyCookieDomains overrides the cookie domains from ConfigMap
	// Value: comma-separated domains (e.g., "example.com,corp.example.com")
	KeyCookieDomains = AnnotationPrefix + "cookie-domains"

	// ===== Routing Overrides (override ConfigMap values) =====

	// KeyRedirectURL overrides the OAuth callback URL from ConfigMap
	// Value: full URL (e.g., "https://myapp.example.com/oauth2/callback")
	// IMPORTANT: Usually needs to be set per-service
	KeyRedirectURL = AnnotationPrefix + "redirect-url"

	// KeyExtraJWTIssuers overrides/adds extra JWT issuers from ConfigMap
	// Value: comma-separated "issuer=audience" pairs
	// Example: "https://issuer1.com=api,https://issuer2.com=api"
	KeyExtraJWTIssuers = AnnotationPrefix + "extra-jwt-issuers"

	// ===== Header Overrides (override ConfigMap values) =====

	// KeyPassAccessToken overrides pass-access-token from ConfigMap
	// Value: "true" or "false"
	KeyPassAccessToken = AnnotationPrefix + "pass-access-token"

	// KeySetXAuthRequest overrides set-xauthrequest from ConfigMap
	// Value: "true" or "false"
	KeySetXAuthRequest = AnnotationPrefix + "set-xauthrequest"

	// KeyPassAuthorizationHeader overrides pass-authorization-header from ConfigMap
	// Value: "true" or "false"
	KeyPassAuthorizationHeader = AnnotationPrefix + "pass-authorization-header"

	// ===== Behavior Overrides (override ConfigMap values) =====

	// KeySkipProviderButton overrides skip-provider-button from ConfigMap
	// Value: "true" or "false"
	KeySkipProviderButton = AnnotationPrefix + "skip-provider-button"

	// ===== Provider Overrides (rarely needed, but available) =====

	// KeyProvider overrides the OAuth2 provider from ConfigMap
	// Value: "oidc", "google", "github", etc.
	// Use case: Testing with different provider, multi-tenant setups
	KeyProvider = AnnotationPrefix + "provider"

	// KeyOIDCIssuerURL overrides the OIDC issuer URL from ConfigMap
	// Value: full URL (e.g., "https://auth.example.com/realms/myrealm")
	// Use case: Per-service realm in multi-tenant Keycloak
	KeyOIDCIssuerURL = AnnotationPrefix + "oidc-issuer-url"

	// KeyOIDCGroupsClaim overrides the groups claim name from ConfigMap
	// Value: claim name (e.g., "groups", "roles")
	KeyOIDCGroupsClaim = AnnotationPrefix + "oidc-groups-claim"

	// ===== Cookie Overrides =====

	// KeyCookieSecure overrides the cookie secure flag from ConfigMap
	// Value: "true" or "false"
	// Use case: Development/testing without HTTPS
	KeyCookieSecure = AnnotationPrefix + "cookie-secure"

	// ===== Container Overrides =====

	// KeyProxyImage overrides the oauth2-proxy image from ConfigMap
	// Value: full image reference (e.g., "quay.io/oauth2-proxy/oauth2-proxy:v7.6.0")
	// Use case: Testing new versions, using custom builds
	KeyProxyImage = AnnotationPrefix + "proxy-image"

	// KeyPingPath overrides the oauth2-proxy ping/healthz endpoint path
	// Value: path (e.g., "/oauth2/ping")
	// Default: "/ping" (oauth2-proxy default)
	// Use case: When app's health check path conflicts with oauth2-proxy's default
	KeyPingPath = AnnotationPrefix + "ping-path"

	// KeyReadyPath overrides the oauth2-proxy ready endpoint path
	// Value: path (e.g., "/oauth2/ready")
	// Default: "/ready" (oauth2-proxy default)
	// Use case: When app's health check path conflicts with oauth2-proxy's default
	KeyReadyPath = AnnotationPrefix + "ready-path"

	// ===== Upstream Override =====

	// KeyUpstream overrides the default upstream URL
	// Value: full URL (e.g., "http://127.0.0.1:8080", "http://other-service:80")
	// Use case: Route to different backend, external service, or specific path
	// When set, this REPLACES the auto-calculated upstream from port mapping
	KeyUpstream = AnnotationPrefix + "upstream"

	// ===== Secret Provider Class (CSI Driver) =====

	// KeySecretProviderClass specifies a SecretProviderClass for CSI secrets driver
	// Value: name of a SecretProviderClass resource (e.g., "oauth2-proxy-vault-secrets")
	// When set, secrets and config are read from files mounted via the CSI driver
	// instead of from Kubernetes Secrets or annotations.
	//
	// The CSI volume is mounted at /etc/oauth2-proxy/conf.d/ in the oauth2-proxy sidecar.
	// Files in that directory override config values by matching annotation key names:
	//   - client-secret -> --client-secret-file (special handling)
	//   - cookie-secret -> --cookie-secret-file (special handling)
	//   - client-id -> --client-id=<file contents>
	//   - redirect-url -> --redirect-url=<file contents>
	//   - etc.
	//
	// Precedence (highest to lowest):
	//   1. Files in secret-provider-class mount
	//   2. Pod annotations
	//   3. ConfigMap
	KeySecretProviderClass = AnnotationPrefix + "secret-provider-class"

	// KeyEnvSecret specifies a Secret to use for env var injection
	// Value: name of a Secret resource (e.g., "oauth2-proxy-env")
	// When set, fields with "fromEnv" source will generate env var entries
	// that read from this Secret. The Secret keys should match annotation names
	// (e.g., "client-id", "provider", "oidc-issuer-url").
	//
	// Example Secret:
	//   data:
	//     client-id: bXktY2xpZW50  # base64 of "my-client"
	//     provider: b2lkYw==       # base64 of "oidc"
	//
	// This generates env vars like:
	//   - name: OAUTH2_PROXY_CLIENT_ID
	//     valueFrom:
	//       secretKeyRef:
	//         name: oauth2-proxy-env
	//         key: client-id
	KeyEnvSecret = AnnotationPrefix + "env-secret"

	// KeyExtraEnv specifies additional env vars to inject from the env-secret
	// Value: comma-separated "secretKey:ENV_VAR_NAME" pairs
	// Example: "project-id:PROJECT_ID,custom-value:MY_CUSTOM_VAR"
	//
	// This allows injecting arbitrary env vars that oauth2-proxy expands at runtime.
	// Reference them in literal annotation values using ${VAR_NAME} syntax.
	// Requires env-secret to be set.
	//
	// Example usage:
	//   annotations:
	//     spacemule.net/oauth2-proxy.env-secret: "my-secrets"
	//     spacemule.net/oauth2-proxy.extra-env: "project-id:PROJECT_ID"
	//     spacemule.net/oauth2-proxy.allowed-groups: "${PROJECT_ID}:admin,${PROJECT_ID}:family"
	KeyExtraEnv = AnnotationPrefix + "extra-env"

	// KeyEnvFile specifies a file path to source before starting oauth2-proxy
	// Value: absolute file path (e.g., "/vault/secrets/env")
	//
	// When set, the container command becomes:
	//   /bin/sh -c "source /vault/secrets/env && exec /bin/oauth2-proxy ..."
	//
	// This is useful for Vault Agent Injector which writes files, not env vars.
	// The file should contain shell export statements:
	//   export OAUTH2_PROXY_CLIENT_ID="my-client"
	//   export OAUTH2_PROXY_OIDC_ISSUER_URL="https://auth.example.com"
	//
	// Use with "fromEnv" annotation values to skip flag generation:
	//   annotations:
	//     spacemule.net/oauth2-proxy.env-file: "/vault/secrets/env"
	//     spacemule.net/oauth2-proxy.client-id: "fromEnv"
	KeyEnvFile = AnnotationPrefix + "env-file"
)

// UpstreamTLSMode represents the TLS verification mode for upstream connections
type UpstreamTLSMode string

const (
	// UpstreamTLSSecure verifies upstream TLS certificates (default)
	UpstreamTLSSecure UpstreamTLSMode = "https"

	// UpstreamTLSInsecure skips TLS verification for upstream connections
	// Use for pods that terminate TLS internally with self-signed certs
	UpstreamTLSInsecure UpstreamTLSMode = "https-insecure"

	// UpstreamNoTLS connects via http to the upstream
	UpstreamNoTLS UpstreamTLSMode = "http"
)

// ValueSourceType represents how a configuration value should be resolved
type ValueSourceType string

const (
	// ValueSourceLiteral means the value is used directly as provided
	ValueSourceLiteral ValueSourceType = "literal"

	// ValueSourceFile means the value should be read from a CSI-mounted file
	// Only valid for: client-secret, cookie-secret
	// Results in: --client-secret-file=/etc/oauth2-proxy/conf.d/client-secret
	ValueSourceFile ValueSourceType = "file"

	// ValueSourceEnv means oauth2-proxy should read the value from environment variables
	// The webhook skips generating the flag; oauth2-proxy reads OAUTH2_PROXY_* vars automatically
	// Example: client-id: "fromEnv" -> no --client-id flag, oauth2-proxy reads OAUTH2_PROXY_CLIENT_ID
	ValueSourceEnv ValueSourceType = "fromEnv"
)

// ValueSource represents a configuration value with its source type
// This allows values to come from literals, CSI-mounted files, or environment variables
type ValueSource struct {
	// Type indicates how this value should be resolved
	Type ValueSourceType

	// Value holds the literal value when Type is ValueSourceLiteral
	// Empty when Type is ValueSourceFile or ValueSourceEnv
	Value string
}

// ParseValueSource parses an annotation value into a ValueSource
//
// Supported formats:
//   - "file"         -> ValueSource{Type: ValueSourceFile} (uses default CSI path)
//   - "file:/path"   -> ValueSource{Type: ValueSourceFile, Value: "/path"} (explicit path)
//   - "fromEnv"      -> ValueSource{Type: ValueSourceEnv}
//   - anything else  -> ValueSource{Type: ValueSourceLiteral, Value: <the value>}
func ParseValueSource(value string) ValueSource {
	switch {
	case value == "file":
		return ValueSource{Type: ValueSourceFile}
	case strings.HasPrefix(value, "file:"):
		// Explicit file path: "file:/vault/secrets/client-secret"
		return ValueSource{Type: ValueSourceFile, Value: strings.TrimPrefix(value, "file:")}
	case value == "fromEnv":
		return ValueSource{Type: ValueSourceEnv}
	default:
		return ValueSource{Type: ValueSourceLiteral, Value: value}
	}
}

// BoolValueSource represents a boolean configuration value with its source type
type BoolValueSource struct {
	Type  ValueSourceType
	Value bool
}

// IsSet returns true if this was explicitly set
func (bvs BoolValueSource) IsSet() bool {
	return bvs.Type != ""
}

// ParseBoolValueSource parses an annotation value into a BoolValueSource
//
// Supported formats:
//   - "fromEnv" -> BoolValueSource{Type: ValueSourceEnv}
//   - "true"/"1" -> BoolValueSource{Type: ValueSourceLiteral, Value: true}
//   - "false"/"0" -> BoolValueSource{Type: ValueSourceLiteral, Value: false}
func ParseBoolValueSource(value string) (BoolValueSource, error) {
	var parsed bool
	if value == "fromEnv" {
		return BoolValueSource{Type: ValueSourceEnv}, nil
	}
	switch strings.ToLower(value) {
	case "true":
		parsed = true
	case "false":
		parsed = false
	case "1":
		parsed = true
	case "0":
		parsed = false
	default:
		return BoolValueSource{}, fmt.Errorf("invalid boolean value %q", value)
	}
	return BoolValueSource{
		Type: ValueSourceLiteral,
		Value: parsed,
	}, nil
}

// StringSliceValueSource represents a string slice configuration value with its source type
type StringSliceValueSource struct {
	Type   ValueSourceType
	Values []string
}

// IsSet returns true if this was explicitly set
func (ssvs StringSliceValueSource) IsSet() bool {
	return ssvs.Type != ""
}

// ParseStringSliceValueSource parses an annotation value into a StringSliceValueSource
//
// Supported formats:
//   - "fromEnv" -> StringSliceValueSource{Type: ValueSourceEnv}
//   - comma-separated values -> StringSliceValueSource{Type: ValueSourceLiteral, Values: [...]}
func ParseStringSliceValueSource(value string) StringSliceValueSource {
	if value == "fromEnv" {
		return StringSliceValueSource{Type: ValueSourceEnv}
	}
	parsed := []string{}

	splits := strings.Split(value, ",")
	for _, split := range splits {
		if trimmed := strings.TrimSpace(split); trimmed != "" {
			parsed = append(parsed, trimmed)
		}
	}

	return StringSliceValueSource{
		Type: ValueSourceLiteral,
		Values: parsed,
	}
}

// IsSet returns true if this ValueSource was explicitly set (not zero value)
func (vs ValueSource) IsSet() bool {
	return vs.Type != ""
}

// IsLiteral returns true if this is a literal value
func (vs ValueSource) IsLiteral() bool {
	return vs.Type == ValueSourceLiteral
}

// IsFile returns true if this value should be read from a file
func (vs ValueSource) IsFile() bool {
	return vs.Type == ValueSourceFile
}

// IsFromEnv returns true if oauth2-proxy should read this from env vars
func (vs ValueSource) IsFromEnv() bool {
	return vs.Type == ValueSourceEnv
}

// parseExtraEnv parses a "secretKey:ENV_VAR,secretKey2:ENV_VAR2" format string
// into a map[string]string where keys are secret keys and values are env var names
func parseExtraEnv(value string) (map[string]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	result := make(map[string]string)
	pairs := strings.Split(value, ",")

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid %s format: %q (expected secretKey:ENV_VAR_NAME)", KeyExtraEnv, pair)
		}

		secretKey := strings.TrimSpace(parts[0])
		envVarName := strings.TrimSpace(parts[1])

		if secretKey == "" || envVarName == "" {
			return nil, fmt.Errorf("invalid %s format: %q (empty key or env var name)", KeyExtraEnv, pair)
		}

		result[secretKey] = envVarName
	}

	return result, nil
}

// Config holds parsed annotation values for a pod
// This includes both annotation-only settings and ConfigMap overrides
type Config struct {
	// ===== Core Settings =====

	// Enabled indicates whether oauth2-proxy should be injected
	Enabled bool

	// ConfigMapName is the name of the ConfigMap containing oauth2-proxy settings
	ConfigMapName string

	// BlockDirectAccess indicates whether or not to inject an iptables initContainer to protect the service
	BlockDirectAccess bool

	// SecretProviderClass is the name of the SecretProviderClass for CSI secrets driver
	// When set, a CSI volume is mounted and config values are read from files
	SecretProviderClass string

	// EnvSecret is the name of a Secret to use for env var injection
	// When set, fields with "fromEnv" source will generate env var entries
	// that read from this Secret using the annotation name as the key
	EnvSecret string

	// ExtraEnv is a map of secret keys to env var names for arbitrary env var injection
	// Parsed from "secretKey:ENV_VAR_NAME,secretKey2:ENV_VAR_NAME2" format
	// Requires EnvSecret to be set
	ExtraEnv map[string]string

	// EnvFile is the path to a file to source before starting oauth2-proxy
	// When set, the container uses: /bin/sh -c "source <path> && exec /bin/oauth2-proxy ..."
	// Useful for Vault Agent Injector which writes files containing export statements
	EnvFile string

	// ===== Pod-Specific Settings (annotation-only, no fromEnv support) =====
	// These are inherently per-pod and wouldn't make sense to read from env vars

	// ProtectedPort is the name of the port that should be proxied
	ProtectedPort string

	// IgnorePaths is the list of paths that should NOT be proxied (pod-specific routing)
	IgnorePaths []string

	// APIPaths is the list of paths that should not offer login and instead require a JWT
	APIPaths []string

	// UpstreamTLS is the TLS mode for upstream connections
	UpstreamTLS UpstreamTLSMode

	// PingPath is the path for oauth2-proxy's ping/healthz endpoint
	PingPath string

	// ReadyPath is the path for oauth2-proxy's ready endpoint
	ReadyPath string

	// ===== ConfigMap Overrides =====
	// These fields override the corresponding ConfigMap values when set.
	// All support "fromEnv" to let oauth2-proxy read from environment variables.

	// Overrides contains all the fields that can override ConfigMap values
	Overrides ConfigOverrides
}

// ConfigOverrides holds annotation values that override ConfigMap settings
//
// All non-pod-specific fields support "fromEnv" to let oauth2-proxy read from
// environment variables at runtime instead of passing flags.
//
// Value source types:
//   - "file"    -> read from CSI-mounted file (only client-secret, cookie-secret)
//   - "fromEnv" -> oauth2-proxy reads from OAUTH2_PROXY_* env var (webhook skips the flag)
//   - <value>   -> literal value passed as --flag=<value>
type ConfigOverrides struct {
	// ===== Provider Overrides =====

	// Provider overrides the OAuth2 provider type
	Provider ValueSource

	// OIDCIssuerURL overrides the OIDC issuer URL
	OIDCIssuerURL ValueSource

	// OIDCGroupsClaim overrides the OIDC groups claim name
	OIDCGroupsClaim ValueSource

	// Scope overrides the scope
	Scope ValueSource

	// ValidateURL overrides the validate-url
	ValidateURL ValueSource

	// ===== Identity Overrides =====

	// ClientID overrides the OAuth2 client ID
	ClientID ValueSource

	// ClientSecretRef overrides the client secret reference
	// When literal: Value is "secret-name" or "secret-name:key"
	// When file: uses --client-secret-file pointing to CSI mount
	// When fromEnv: skips flag (oauth2-proxy reads OAUTH2_PROXY_CLIENT_SECRET)
	ClientSecretRef ValueSource

	// PKCEEnabled overrides the PKCE setting
	// This is a boolean abstraction (true → code-challenge-method=S256)
	// Does not support fromEnv - use code-challenge-method annotation instead
	PKCEEnabled *bool

	// CodeChallengeMethod overrides the PKCE code challenge method
	CodeChallengeMethod ValueSource

	// ===== Cookie Overrides =====

	// CookieSecretRef overrides the cookie secret reference
	// When literal: Value is "secret-name" or "secret-name:key"
	// When file: uses --cookie-secret-file pointing to CSI mount
	// When fromEnv: skips flag (oauth2-proxy reads OAUTH2_PROXY_COOKIE_SECRET)
	CookieSecretRef ValueSource

	// CookieDomains overrides cookie domains
	CookieDomains StringSliceValueSource

	// CookieSecure overrides the cookie secure flag
	CookieSecure BoolValueSource

	// CookieName overrides cookie name
	CookieName ValueSource

	// ===== Authorization Overrides =====

	// EmailDomains overrides allowed email domains
	EmailDomains StringSliceValueSource

	// AllowedGroups overrides allowed groups
	AllowedGroups StringSliceValueSource

	// WhitelistDomains overrides allowed domains for redirects
	WhitelistDomains StringSliceValueSource

	// ===== Routing Overrides =====

	// RedirectURL overrides the OAuth callback URL
	RedirectURL ValueSource

	// ExtraJWTIssuers overrides extra JWT issuers
	ExtraJWTIssuers StringSliceValueSource

	// Upstream overrides the auto-calculated upstream URL
	Upstream ValueSource

	// ===== Header Overrides =====

	// PassAccessToken overrides pass-access-token
	PassAccessToken BoolValueSource

	// SetXAuthRequest overrides set-xauthrequest
	SetXAuthRequest BoolValueSource

	// PassAuthorizationHeader overrides pass-authorization-header
	PassAuthorizationHeader BoolValueSource

	// ===== Behavior Overrides =====

	// SkipProviderButton overrides skip-provider-button
	SkipProviderButton BoolValueSource

	// SkipJWTBearerTokens overrides skip-jwt-bearer-tokens
	SkipJWTBearerTokens BoolValueSource

	// ===== Container Overrides =====

	// ProxyImage overrides the oauth2-proxy container image
	// This is a plain *string (not ValueSource) because it's used at pod creation
	// time by the webhook, not by oauth2-proxy at runtime. "fromEnv" makes no sense here.
	ProxyImage *string
}

// Parser defines the interface for parsing pod annotations
type Parser interface {
	// Parse extracts oauth2-proxy configuration from pod annotations
	Parse(annotations map[string]string) (*Config, error)
}

// AnnotationParser implements Parser for oauth2-proxy annotations
type AnnotationParser struct{}

func NewParser() *AnnotationParser {
	return &AnnotationParser{}
}

// Parse extracts oauth2-proxy configuration from pod annotations
func (p *AnnotationParser) Parse(annotations map[string]string) (*Config, error) {
	var (
		cfg *Config = &Config{
			IgnorePaths:         []string{},
			APIPaths:            []string{},
			UpstreamTLS:         UpstreamNoTLS,
			Overrides:           ConfigOverrides{},
		}
	)

	if annotations[KeyEnabled] != "true" {
		return &Config{Enabled: false}, nil
	}
	cfg.Enabled = true

	// ConfigMapName is optional - if not set, mutator will use webhook's default
	if v, ok := annotations[KeyConfig]; ok {
		cfg.ConfigMapName = v
	}

	cfg.BlockDirectAccess = false
	if annotations[KeyBlockDirectAccess] == "true" {
		cfg.BlockDirectAccess = true
	}

	if v, ok := annotations[KeyProvider]; ok {
		cfg.Overrides.Provider = ParseValueSource(v)
	}

	if v, ok := annotations[KeyOIDCIssuerURL]; ok {
		cfg.Overrides.OIDCIssuerURL = ParseValueSource(v)
	}

	if v, ok := annotations[KeyOIDCGroupsClaim]; ok {
		cfg.Overrides.OIDCGroupsClaim = ParseValueSource(strings.TrimSpace(v))
	}

	if v, ok := annotations[KeyProxyImage]; ok {
		s := strings.TrimSpace(v)
		cfg.Overrides.ProxyImage = &s
	}

	if v, ok := annotations[KeyPingPath]; ok {
		cfg.PingPath = strings.TrimSpace(v)
	}

	if v, ok := annotations[KeyReadyPath]; ok {
		cfg.ReadyPath = strings.TrimSpace(v)
	}

	if v, ok := annotations[KeyUpstream]; ok {
		cfg.Overrides.Upstream = ParseValueSource(v)
	}

	if v, ok := annotations[KeyCookieSecure]; ok {
		b, err := ParseBoolValueSource(v)
		if err != nil {
			return nil, err
		}
		cfg.Overrides.CookieSecure = b
	}

	if v, ok := annotations[KeyProtectedPort]; ok {
		cfg.ProtectedPort = strings.TrimSpace(v)
	}

	if v, ok := annotations[KeyIgnorePaths]; ok {
		cfg.IgnorePaths = parsePaths(v)
	}

	if v, ok := annotations[KeyAPIPaths]; ok {
		cfg.APIPaths = parsePaths(v)
	}

	if v, ok := annotations[KeySkipJWTBearerTokens]; ok {
		b, err := ParseBoolValueSource(v)
		if err != nil {
			return nil, err
		}
		cfg.Overrides.SkipJWTBearerTokens = b
	}

	if v, ok := annotations[KeySecretProviderClass]; ok {
		cfg.SecretProviderClass = strings.TrimSpace(v)
	}

	if v, ok := annotations[KeyEnvSecret]; ok {
		cfg.EnvSecret = strings.TrimSpace(v)
	}

	if v, ok := annotations[KeyExtraEnv]; ok {
		extraEnv, err := parseExtraEnv(v)
		if err != nil {
			return nil, err
		}
		cfg.ExtraEnv = extraEnv
	}

	if v, ok := annotations[KeyEnvFile]; ok {
		cfg.EnvFile = strings.TrimSpace(v)
	}

	if v, ok := annotations[KeyUpstreamTLS]; ok {
		if v != string(UpstreamNoTLS) && v != string(UpstreamTLSInsecure) && v != string(UpstreamTLSSecure) {
			return nil, fmt.Errorf("invalid upstream-tls value: %q (must be %s, %s, or %s)", v, UpstreamNoTLS, UpstreamTLSInsecure, UpstreamTLSSecure)
		}
		cfg.UpstreamTLS = UpstreamTLSMode(v)
	}

	if v, ok := annotations[KeyClientID]; ok {
		cfg.Overrides.ClientID = ParseValueSource(v)
	}

	if v, ok := annotations[KeyClientSecretRef]; ok {
		cfg.Overrides.ClientSecretRef = ParseValueSource(v)
	}

	if v, ok := annotations[KeyCookieSecretRef]; ok {
		cfg.Overrides.CookieSecretRef = ParseValueSource(v)
	}

	if v, ok := annotations[KeyScope]; ok {
		cfg.Overrides.Scope = ParseValueSource(v)
	}

	if v, ok := annotations[KeyValidateURL]; ok {
		cfg.Overrides.ValidateURL = ParseValueSource(v)
	}

	if v, ok := annotations[KeyPKCEEnabled]; ok {
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return nil, fmt.Errorf("invalid %s value: %q (must be true or false)", KeyPKCEEnabled, v)
		}
		cfg.Overrides.PKCEEnabled = &b
	}

	if v, ok := annotations[KeyCodeChallengeMethod]; ok {
		cfg.Overrides.CodeChallengeMethod = ParseValueSource(strings.TrimSpace(v))
	}

	if v, ok := annotations[KeyEmailDomains]; ok {
		cfg.Overrides.EmailDomains = ParseStringSliceValueSource(v)
	}

	if v, ok := annotations[KeyAllowedGroups]; ok {
		cfg.Overrides.AllowedGroups = ParseStringSliceValueSource(v)
	}

	// if v, ok := annotations[KeyAllowedEmails]; ok {
	// cfg.Overrides.AllowedEmails = parsePaths(v)
	// cfg.Overrides.AllowedEmailsSet = true
	// }

	if v, ok := annotations[KeyWhitelistDomains]; ok {
		cfg.Overrides.WhitelistDomains = ParseStringSliceValueSource(v)
	}

	if v, ok := annotations[KeyCookieName]; ok {
		cfg.Overrides.CookieName = ParseValueSource(strings.TrimSpace(v))
	}

	if v, ok := annotations[KeyCookieDomains]; ok {
		cfg.Overrides.CookieDomains = ParseStringSliceValueSource(v)
	}

	if v, ok := annotations[KeyRedirectURL]; ok {
		cfg.Overrides.RedirectURL = ParseValueSource(v)
	}

	if v, ok := annotations[KeyExtraJWTIssuers]; ok {
		cfg.Overrides.ExtraJWTIssuers = ParseStringSliceValueSource(v)
	}

	if v, ok := annotations[KeyPassAccessToken]; ok {
		b, err := ParseBoolValueSource(v)
		if err != nil {
			return nil, err
		}
		cfg.Overrides.PassAccessToken = b
	}

	if v, ok := annotations[KeySetXAuthRequest]; ok {
		b, err := ParseBoolValueSource(v)
		if err != nil {
			return nil, err
		}
		cfg.Overrides.SetXAuthRequest = b
	}

	if v, ok := annotations[KeyPassAuthorizationHeader]; ok {
		b, err := ParseBoolValueSource(v)
		if err != nil {
			return nil, err
		}
		cfg.Overrides.PassAuthorizationHeader = b
	}

	if v, ok := annotations[KeySkipProviderButton]; ok {
		b, err := ParseBoolValueSource(v)
		if err != nil {
			return nil, err
		}
		cfg.Overrides.SkipProviderButton = b
	}

	return cfg, nil
}

// parseBoolPtr parses a boolean string and returns a pointer
// This distinguishes "not set" (nil) from "set to false" (*false)
func parseBoolPtr(value string) (*bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return nil, nil
	case "true", "1":
		pt := true
		return &pt, nil
	case "false", "0":
		pt := false
		return &pt, nil
	default:
		return nil, fmt.Errorf("invalid boolean value: %q (must be 'true', 'false', '1', or '0')", value)
	}
}

func parsePaths(pathsStr string) []string {
	var result []string
	if pathsStr == "" {
		return result
	}
	for _, path := range strings.Split(pathsStr, ",") {
		result = append(result, strings.TrimSpace(path))
	}
	return result
}

// IsNamedPort returns true if the protected port is specified by name (e.g., "http")
// rather than by number (e.g., "8080").
//
// This distinction matters for mutation behavior:
// - Named port: "takeover" mode - sidecar takes the port name, app port is removed, probes rewritten
// - Numbered port: "service mutation" mode - app keeps its ports, sidecar gets generic port, Service webhook handles routing
func IsNamedPort(protectedPort string) bool {
	for _, c := range protectedPort {
		if c < '0' || c > '9' {
			return true
		}
	}
	return false
}
