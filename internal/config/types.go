package config

import (
	"fmt"

	"github.com/spacemule/oauth2-proxy-injector/internal/annotation"
	corev1 "k8s.io/api/core/v1"
)

// ProxyConfig holds the oauth2-proxy configuration loaded from a ConfigMap
// This represents the BASE settings that will be passed to the oauth2-proxy sidecar.
// Many of these fields can be overridden by pod annotations for per-service customization.
type ProxyConfig struct {
	// Name is the ConfigMap name this was loaded from
	Name string

	// Namespace is where the ConfigMap lives
	Namespace string

	// ===== Provider Settings (ConfigMap only - shared across namespace) =====

	// Provider is the OAuth2 provider (e.g., "oidc", "google", "github")
	Provider string

	// OIDCIssuerURL is the OIDC provider's issuer URL
	// Example: "https://auth.example.com/realms/myrealm"
	OIDCIssuerURL string

	// OIDCGroupsClaim specifies which claim contains group membership
	// Default: "groups" - override if your provider uses a different claim
	OIDCGroupsClaim string

	// Scope specifies OAuth scopes to request
	// Default: "openid email profile" for OIDC
	Scope string

	// ===== Identity Settings (overridable per-service) =====

	// ClientID is the OAuth2 client ID
	// Overridable: Different services may use different OAuth apps
	ClientID string

	// ClientSecretRef references a Secret containing the client secret
	// Format: "secret-name" (key defaults to "client-secret")
	// or "secret-name:key-name" for custom key
	// Overridable: Different services may have different secrets
	ClientSecretRef *SecretRef

	// PKCEEnabled sets whether or not to use PKCE to allow for config without a client_secret
	// Overridable: Some services may use PKCE, others may not
	PKCEEnabled bool

	// ===== Cookie Settings =====

	// CookieSecretRef references a Secret containing the cookie encryption secret
	// oauth2-proxy requires a 16, 24, or 32 byte secret for AES
	// Overridable: Services may need isolated cookie encryption
	CookieSecretRef *SecretRef

	// CookieDomains sets the domain for oauth2-proxy cookies
	// Leave empty for automatic domain detection
	// Overridable: Different services may have different domains
	CookieDomains []string

	// CookieSecure determines if cookies should have the Secure flag
	// Default: true (required for HTTPS)
	CookieSecure bool

	// CookieName is the name of the oauth2-proxy cookie
	// Default: "_oauth2_proxy"
	// Overridable: Prevents cookie collision when multiple proxied services share a domain
	CookieName string

	// ===== Authorization Settings (overridable - services may have different access rules) =====

	// EmailDomains restricts access to specific email domains
	// Example: ["example.com", "corp.example.com"]
	// Use ["*"] to allow all domains
	// Overridable: Different services may allow different domains
	EmailDomains []string

	// AllowedGroups restricts access to users in specific groups
	// Works with providers that support group claims
	// Overridable: Different services may require different groups
	AllowedGroups []string

	// AllowedEmails restricts access to specific email addresses
	// More granular than EmailDomains for sensitive services
	// Overridable: Per-service access lists
	// AllowedEmails []string

	// WhitelistDomains are domains allowed for post-auth redirects
	// Prevents open redirect vulnerabilities
	WhitelistDomains []string

	// ===== Routing Settings (overridable per-service) =====

	// RedirectURL is the OAuth callback URL
	// Example: "https://myapp.example.com/oauth2/callback"
	// Overridable: REQUIRED to be different per-service in most deployments
	RedirectURL string

	// ExtraJWTIssuers allows additional JWT issuers for bearer token auth
	// Format: "issuer=audience" pairs
	// Example: ["https://issuer1.com=api", "https://issuer2.com=api"]
	// Overridable: Different services may accept tokens from different issuers
	ExtraJWTIssuers []string

	// ===== Header/Token Passing Settings (overridable per-service) =====

	// PassAccessToken passes the OAuth access token to upstream via X-Forwarded-Access-Token
	// Overridable: Some upstreams need the access token, others don't
	PassAccessToken bool

	// SetXAuthRequest sets X-Auth-Request-User, X-Auth-Request-Email headers
	// Overridable: Useful when upstream needs user identity
	SetXAuthRequest bool

	// PassAuthorizationHeader passes the OIDC ID token via Authorization: Bearer header
	// Overridable: Some upstreams validate the ID token themselves
	PassAuthorizationHeader bool

	// ===== Behavior Settings =====

	// SkipProviderButton skips the "Sign in with X" button and redirects directly
	// Overridable: UX preference per service
	SkipProviderButton bool

	// ===== Container Settings (ConfigMap only) =====

	// ExtraArgs contains any additional oauth2-proxy arguments
	// These are passed directly to the sidecar container
	// NOT overridable via annotations - use specific fields instead for safety
	ExtraArgs []string

	// ProxyImage is the oauth2-proxy container image to use
	// Default: "quay.io/oauth2-proxy/oauth2-proxy:v7.5.1"
	ProxyImage string

	// ProxyResources specifies resource requests/limits for the sidecar
	// Optional - uses oauth2-proxy defaults if not set
	ProxyResources *corev1.ResourceRequirements
}

// SecretRef references a key in a Kubernetes Secret
type SecretRef struct {
	// Name is the Secret name
	Name string

	// Key is the key within the Secret's data
	// Defaults to a sensible value if not specified
	Key string
}

// ConfigMapKeys defines the expected keys in an oauth2-proxy ConfigMap
// These constants help ensure consistency between config creation and loading
//
// Keys are organized by category:
// - Provider: Core OAuth/OIDC provider settings
// - Identity: Client credentials
// - Cookie: Session cookie configuration
// - Authorization: Access control rules
// - Routing: Redirect and JWT settings
// - Headers: Token passing to upstream
// - Behavior: UX and proxy behavior
// - Container: Sidecar container settings
const (
	// ===== Provider Settings =====

	// CMKeyProvider is the oauth2 provider type (e.g., "oidc", "google", "github")
	CMKeyProvider = "provider"

	// CMKeyOIDCIssuerURL is the OIDC issuer URL
	CMKeyOIDCIssuerURL = "oidc-issuer-url"

	// CMKeyOIDCGroupsClaim specifies the claim containing group membership
	// Default: "groups"
	CMKeyOIDCGroupsClaim = "oidc-groups-claim"

	// CMKeyScope specifies OAuth scopes to request
	CMKeyScope = "scope"

	// ===== Identity Settings (overridable) =====

	// CMKeyClientID is the OAuth2 client ID
	CMKeyClientID = "client-id"

	// CMKeyClientSecretRef references the client secret
	// Format: "secret-name" or "secret-name:key"
	CMKeyClientSecretRef = "client-secret-ref"

	// CMKeyPKCEEnabled is whether or not the proxy is using PKCE without a client secret
	CMKeyPKCEEnabled = "pkce-enabled"

	// ===== Cookie Settings =====

	// CMKeyCookieSecretRef references the cookie secret
	CMKeyCookieSecretRef = "cookie-secret-ref"

	// CMKeyCookieDomains is comma-separated cookie domains
	CMKeyCookieDomains = "cookie-domains"

	// CMKeyCookieSecure is whether cookies require HTTPS
	CMKeyCookieSecure = "cookie-secure"

	// CMKeyCookieName is the name of the oauth2-proxy cookie
	CMKeyCookieName = "cookie-name"

	// ===== Authorization Settings (overridable) =====

	// CMKeyEmailDomains is comma-separated allowed email domains
	CMKeyEmailDomains = "email-domains"

	// CMKeyAllowedGroups is comma-separated allowed groups
	CMKeyAllowedGroups = "allowed-groups"

	// CMKeyAllowedEmails is comma-separated allowed email addresses
	// CMKeyAllowedEmails = "allowed-emails"

	// CMKeyWhitelistDomains is comma-separated domains allowed for redirects
	CMKeyWhitelistDomains = "whitelist-domains"

	// ===== Routing Settings (overridable) =====

	// CMKeyRedirectURL is the OAuth callback URL
	CMKeyRedirectURL = "redirect-url"

	// CMKeyExtraJWTIssuers is comma-separated issuer=audience pairs
	CMKeyExtraJWTIssuers = "extra-jwt-issuers"

	// ===== Header Settings (overridable) =====

	// CMKeyPassAccessToken passes OAuth access token to upstream
	CMKeyPassAccessToken = "pass-access-token"

	// CMKeySetXAuthRequest sets X-Auth-Request-* headers
	CMKeySetXAuthRequest = "set-xauthrequest"

	// CMKeyPassAuthorizationHeader passes OIDC ID token as Authorization header
	CMKeyPassAuthorizationHeader = "pass-authorization-header"

	// ===== Behavior Settings (overridable) =====

	// CMKeySkipProviderButton skips the provider button page
	CMKeySkipProviderButton = "skip-provider-button"

	// ===== Container Settings (not overridable) =====

	// CMKeyExtraArgs is newline-separated extra arguments
	// WARNING: Not overridable via annotations for security
	CMKeyExtraArgs = "extra-args"

	// CMKeyProxyImage is the oauth2-proxy container image
	CMKeyProxyImage = "proxy-image"
)

// DefaultProxyImage is the default oauth2-proxy container image
const DefaultProxyImage = "quay.io/oauth2-proxy/oauth2-proxy:v7.14.2"

// NewEmptyProxyConfig creates an empty ProxyConfig with sensible defaults
// Used for annotation-only mode where no ConfigMap is specified
func NewEmptyProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		ProxyImage:   DefaultProxyImage,
		CookieSecure: true,
	}
}

// EffectiveConfig represents the final, merged configuration after applying
// pod annotation overrides to the base ConfigMap settings.
// This is what actually gets passed to the sidecar builder.
type EffectiveConfig struct {
	// Source tracking for debugging/logging
	ConfigMapName      string
	ConfigMapNamespace string

	// ===== Provider Settings (from ConfigMap only) =====
	Provider        string
	OIDCIssuerURL   string
	OIDCGroupsClaim string
	Scope           string

	// ===== Identity Settings (merged) =====
	ClientID        string
	ClientSecretRef *SecretRef
	PKCEEnabled     bool

	// ===== Cookie Settings (merged) =====
	CookieSecretRef *SecretRef
	CookieDomains   []string
	CookieSecure    bool
	CookieName      string

	// ===== Authorization Settings (merged) =====
	EmailDomains  []string
	AllowedGroups []string
	// AllowedEmails    []string
	WhitelistDomains []string

	// ===== Routing Settings (merged) =====
	RedirectURL     string
	ExtraJWTIssuers []string

	// ===== Header Settings (merged) =====
	PassAccessToken         bool
	SetXAuthRequest         bool
	PassAuthorizationHeader bool

	// ===== Behavior Settings (merged) =====
	SkipProviderButton bool

	// ===== Annotation-Only Settings =====
	// These come only from annotations, not ConfigMap

	BlockDirectAccess   bool
	ProtectedPort       string
	IgnorePaths         []string
	APIPaths            []string
	SkipJWTBearerTokens bool
	UpstreamTLS         annotation.UpstreamTLSMode // "http", "https", "https-insecure"

	// Upstream is an optional override for the auto-calculated upstream URL
	// When empty, the sidecar builder calculates it from port mapping
	// When set, it's used directly (e.g., "http://other-service:8080/api")
	Upstream string

	// ===== Container Settings (from ConfigMap only) =====
	ExtraArgs      []string
	ProxyImage     string
	ProxyResources *corev1.ResourceRequirements
}

// Validation errors that can occur when loading config
type ConfigError struct {
	ConfigMap string
	Field     string
	Message   string
}

// Error implements the error interface for ConfigError
func (e *ConfigError) Error() string {
	return fmt.Sprintf("config %s: field %s: %s", e.ConfigMap, e.Field, e.Message)
}

// ValidationError represents a validation failure with context
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for %s=%q: %s", e.Field, e.Value, e.Message)
}
