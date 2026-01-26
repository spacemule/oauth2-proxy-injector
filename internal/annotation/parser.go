package annotation

import (
	"fmt"
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

	// KeyPKCEEnabled overrides PKCE setting from ConfigMap
	// Value: "true" or "false"
	KeyPKCEEnabled = AnnotationPrefix + "pkce-enabled"

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

	// ===== Port/Routing Settings (annotation-only) =====

	// ProtectedPort is the name of the port that should be proxied
	ProtectedPort string

	// IgnorePaths is the list of paths that should NOT be proxied
	IgnorePaths []string

	// APIPaths is the list of paths that should not offer login and instead require a JWT
	APIPaths []string

	// SkipJWTBearerTokens skips login when bearer tokens are provided
	// Defaults to false (i.e. do not skip login)
	SkipJWTBearerTokens bool

	// UpstreamTLS is the TLS mode for upstream connections
	UpstreamTLS UpstreamTLSMode

	// PingPath is the path for oauth2-proxy's ping/healthz endpoint
	// Default: "/ping" (oauth2-proxy default)
	PingPath string

	// ReadyPath is the path for oauth2-proxy's ready endpoint
	// Default: "/ready" (oauth2-proxy default)
	ReadyPath string

	// ===== ConfigMap Overrides =====
	// These fields override the corresponding ConfigMap values when set.
	// Use pointer types or "set" flags to distinguish "not set" from "set to empty/false"

	// Overrides contains all the fields that can override ConfigMap values
	Overrides ConfigOverrides
}

// ConfigOverrides holds annotation values that override ConfigMap settings
// Pointer types are used so nil means "use ConfigMap value" vs empty/false meaning "override to empty/false"
type ConfigOverrides struct {
	// ===== Identity Overrides =====

	// ClientID overrides the OAuth2 client ID
	ClientID *string

	// ClientSecretRef overrides the client secret reference
	ClientSecretRef *string

	// CookieSecretRef overrides the cookie secret reference
	CookieSecretRef *string

	// Scope overrides the scope
	Scope *string

	// PKCEEnabled overrides the PKCE setting
	PKCEEnabled *bool

	// ===== Authorization Overrides =====

	// EmailDomains overrides allowed email domains
	// nil = use ConfigMap, empty slice = explicitly no domains allowed
	EmailDomains    []string
	EmailDomainsSet bool

	// AllowedGroups overrides allowed groups
	AllowedGroups    []string
	AllowedGroupsSet bool

	// AllowedEmails overrides allowed email addresses
	// AllowedEmails []string
	// AllowedEmailsSet bool

	// WhitelistDomains overrides allowed domains
	WhitelistDomains    []string
	WhitelistDomainsSet bool

	// CookieName overrides cookie name
	CookieName *string

	// CookieDomains overrides cookie domains
	CookieDomains    []string
	CookieDomainsSet bool

	// ===== Routing Overrides =====

	// RedirectURL overrides the OAuth callback URL
	RedirectURL *string

	// ExtraJWTIssuers overrides extra JWT issuers
	ExtraJWTIssuers    []string
	ExtraJWTIssuersSet bool

	// ===== Header Overrides =====

	// PassAccessToken overrides pass-access-token
	PassAccessToken *bool

	// SetXAuthRequest overrides set-xauthrequest
	SetXAuthRequest *bool

	// PassAuthorizationHeader overrides pass-authorization-header
	PassAuthorizationHeader *bool

	// ===== Behavior Overrides =====

	// SkipProviderButton overrides skip-provider-button
	SkipProviderButton *bool

	// ===== Provider Overrides (rarely needed) =====

	// Provider overrides the OAuth2 provider type
	Provider *string

	// OIDCIssuerURL overrides the OIDC issuer URL
	OIDCIssuerURL *string

	// OIDCGroupsClaim overrides the OIDC groups claim name
	OIDCGroupsClaim *string

	// ===== Cookie Overrides =====

	// CookieSecure overrides the cookie secure flag
	CookieSecure *bool

	// ===== Container Overrides =====

	// ProxyImage overrides the oauth2-proxy container image
	ProxyImage *string

	// ===== Upstream Override =====

	// Upstream overrides the auto-calculated upstream URL
	// When set, replaces the default http://127.0.0.1:<port> behavior
	Upstream *string
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
			SkipJWTBearerTokens: false,
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
		s := strings.TrimSpace(v)
		cfg.Overrides.Provider = &s
	}

	if v, ok := annotations[KeyOIDCIssuerURL]; ok {
		s := strings.TrimSpace(v)
		cfg.Overrides.OIDCIssuerURL = &s
	}

	if v, ok := annotations[KeyOIDCGroupsClaim]; ok {
		s := strings.TrimSpace(v)
		cfg.Overrides.OIDCGroupsClaim = &s
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
		s := strings.TrimSpace(v)
		cfg.Overrides.Upstream = &s
	}

	if v, ok := annotations[KeyCookieSecure]; ok {
		b, err := parseBoolPtr(v)
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
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1":
			cfg.SkipJWTBearerTokens = true
		case "false", "0":
			cfg.SkipJWTBearerTokens = false
		default:
			return nil, fmt.Errorf("invalid skip-jwt value: %q (must be 'true', 'false', '1', or '0')", v)
		}
	}

	if v, ok := annotations[KeyUpstreamTLS]; ok {
		if v != string(UpstreamNoTLS) && v != string(UpstreamTLSInsecure) && v != string(UpstreamTLSSecure) {
			return nil, fmt.Errorf("invalid upstream-tls value: %q (must be %s, %s, or %s)", v, UpstreamNoTLS, UpstreamTLSInsecure, UpstreamTLSSecure)
		}
		cfg.UpstreamTLS = UpstreamTLSMode(v)
	}

	if v, ok := annotations[KeyClientID]; ok {
		s := strings.TrimSpace(v)
		cfg.Overrides.ClientID = &s
	}

	if v, ok := annotations[KeyClientSecretRef]; ok {
		s := strings.TrimSpace(v)
		cfg.Overrides.ClientSecretRef = &s
	}

	if v, ok := annotations[KeyCookieSecretRef]; ok {
		s := strings.TrimSpace(v)
		cfg.Overrides.CookieSecretRef = &s
	}

	if v, ok := annotations[KeyScope]; ok {
		s := strings.TrimSpace(v)
		cfg.Overrides.Scope = &s
	}

	if v, ok := annotations[KeyPKCEEnabled]; ok {
		b, err := parseBoolPtr(v)
		if err != nil {
			return nil, err
		}
		cfg.Overrides.PKCEEnabled = b
	}

	if v, ok := annotations[KeyEmailDomains]; ok {
		cfg.Overrides.EmailDomains = parsePaths(v)
		cfg.Overrides.EmailDomainsSet = true
	}

	if v, ok := annotations[KeyAllowedGroups]; ok {
		cfg.Overrides.AllowedGroups = parsePaths(v)
		cfg.Overrides.AllowedGroupsSet = true
	}

	// if v, ok := annotations[KeyAllowedEmails]; ok {
	// cfg.Overrides.AllowedEmails = parsePaths(v)
	// cfg.Overrides.AllowedEmailsSet = true
	// }

	if v, ok := annotations[KeyWhitelistDomains]; ok {
		cfg.Overrides.WhitelistDomains = parsePaths(v)
		cfg.Overrides.WhitelistDomainsSet = true
	}

	if v, ok := annotations[KeyCookieName]; ok {
		s := strings.TrimSpace(v)
		cfg.Overrides.CookieName = &s
	}

	if v, ok := annotations[KeyCookieDomains]; ok {
		cfg.Overrides.CookieDomains = parsePaths(v)
		cfg.Overrides.CookieDomainsSet = true
	}

	if v, ok := annotations[KeyRedirectURL]; ok {
		s := strings.TrimSpace(v)
		cfg.Overrides.RedirectURL = &s
	}

	if v, ok := annotations[KeyExtraJWTIssuers]; ok {
		cfg.Overrides.ExtraJWTIssuers = parsePaths(v)
		cfg.Overrides.ExtraJWTIssuersSet = true
	}

	if v, ok := annotations[KeyPassAccessToken]; ok {
		b, err := parseBoolPtr(v)
		if err != nil {
			return nil, err
		}
		cfg.Overrides.PassAccessToken = b
	}

	if v, ok := annotations[KeySetXAuthRequest]; ok {
		b, err := parseBoolPtr(v)
		if err != nil {
			return nil, err
		}
		cfg.Overrides.SetXAuthRequest = b
	}

	if v, ok := annotations[KeyPassAuthorizationHeader]; ok {
		b, err := parseBoolPtr(v)
		if err != nil {
			return nil, err
		}
		cfg.Overrides.PassAuthorizationHeader = b
	}

	if v, ok := annotations[KeySkipProviderButton]; ok {
		b, err := parseBoolPtr(v)
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
