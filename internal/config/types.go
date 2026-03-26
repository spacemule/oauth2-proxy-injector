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

	// ===== Provider Settings (shared across namespace) =====

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

	// ValidateURL specifies the validation URL for opaque tokens
	ValidateURL string

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

	// CodeChallengeMethod specifies the PKCE code challenge method ("S256" or "plain")
	// If PKCEEnabled is true and this is empty, defaults to "S256"
	// Overridable: Allows custom PKCE method per service
	CodeChallengeMethod string

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

// SourcedValue holds a string value along with its source type
// This allows tracking whether a value is literal, from env, or from file
type SourcedValue struct {
	Value  string
	Source annotation.ValueSourceType
}

// IsFromEnv returns true if oauth2-proxy should read this from env vars
func (sv SourcedValue) IsFromEnv() bool {
	return sv.Source == annotation.ValueSourceEnv
}

// IsFromFile returns true if this should be read from a CSI-mounted file
func (sv SourcedValue) IsFromFile() bool {
	return sv.Source == annotation.ValueSourceFile
}

// IsLiteral returns true if this is a literal value (or unset, defaulting to literal)
func (sv SourcedValue) IsLiteral() bool {
	return sv.Source == annotation.ValueSourceLiteral || sv.Source == ""
}

// SourcedSecretRef holds a SecretRef along with its source type
// When Source is ValueSourceFile or ValueSourceEnv, Ref will be nil
type SourcedSecretRef struct {
	Ref    *SecretRef
	Source annotation.ValueSourceType
}

// IsFromEnv returns true if oauth2-proxy should read this from env vars
func (ssr SourcedSecretRef) IsFromEnv() bool {
	return ssr.Source == annotation.ValueSourceEnv
}

// IsFromFile returns true if this should be read from a CSI-mounted file
func (ssr SourcedSecretRef) IsFromFile() bool {
	return ssr.Source == annotation.ValueSourceFile
}

// IsLiteral returns true if this uses a SecretRef (or unset, defaulting to literal)
func (ssr SourcedSecretRef) IsLiteral() bool {
	return ssr.Source == annotation.ValueSourceLiteral || ssr.Source == ""
}

// SourcedBool holds a bool value along with its source type
// This allows booleans to come from literals or environment variables
type SourcedBool struct {
	Value  bool
	Source annotation.ValueSourceType
}

// IsFromEnv returns true if oauth2-proxy should read this from env vars
func (sb SourcedBool) IsFromEnv() bool {
	return sb.Source == annotation.ValueSourceEnv
}

// IsLiteral returns true if this is a literal value (or unset, defaulting to literal)
func (sb SourcedBool) IsLiteral() bool {
	return sb.Source == annotation.ValueSourceLiteral || sb.Source == ""
}

// SourcedStringSlice holds a string slice along with its source type
// This allows slices to come from literals or environment variables
type SourcedStringSlice struct {
	Values []string
	Source annotation.ValueSourceType
	Set    bool // true if explicitly set via annotation (even if empty)
}

// IsFromEnv returns true if oauth2-proxy should read this from env vars
func (ss SourcedStringSlice) IsFromEnv() bool {
	return ss.Source == annotation.ValueSourceEnv
}

// IsLiteral returns true if this is a literal value (or unset, defaulting to literal)
func (ss SourcedStringSlice) IsLiteral() bool {
	return ss.Source == annotation.ValueSourceLiteral || ss.Source == ""
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

	// CMKeyValidateURL specifies validation URL for opaque tokens
	CMKeyValidateURL = "validate-url"

	// ===== Identity Settings (overridable) =====

	// CMKeyClientID is the OAuth2 client ID
	CMKeyClientID = "client-id"

	// CMKeyClientSecretRef references the client secret
	// Format: "secret-name" or "secret-name:key"
	CMKeyClientSecretRef = "client-secret-ref"

	// CMKeyPKCEEnabled is whether or not the proxy is using PKCE without a client secret
	CMKeyPKCEEnabled = "pkce-enabled"

	// CMKeyCodeChallengeMethod is the PKCE code challenge method
	CMKeyCodeChallengeMethod = "code-challenge-method"

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
//
// Fields that support dynamic value sources use Sourced* types.
// The Source field indicates how the value should be resolved:
//   - ValueSourceLiteral: use the value directly as a flag argument
//   - ValueSourceFile: use --*-file flag pointing to CSI mount (secrets only)
//   - ValueSourceEnv: skip the flag, oauth2-proxy reads from OAUTH2_PROXY_* env var
//
// Pod-specific fields (annotation-only) do NOT support fromEnv because they
// are inherently per-pod settings that wouldn't make sense to read from env.
type EffectiveConfig struct {
	// Source tracking for debugging/logging
	ConfigMapName      string
	ConfigMapNamespace string

	// ===== Provider Settings (merged, supports fromEnv) =====

	Provider        SourcedValue
	OIDCIssuerURL   SourcedValue
	OIDCGroupsClaim SourcedValue
	Scope           SourcedValue
	ValidateURL     SourcedValue

	// ===== Identity Settings (merged, supports fromEnv) =====

	ClientID            SourcedValue
	ClientSecret        SourcedSecretRef
	PKCEEnabled         bool // boolean abstraction, doesn't support fromEnv
	CodeChallengeMethod SourcedValue

	// ===== Cookie Settings (merged, supports fromEnv) =====

	CookieSecret  SourcedSecretRef
	CookieDomains SourcedStringSlice
	CookieSecure  SourcedBool
	CookieName    SourcedValue

	// ===== Authorization Settings (merged, supports fromEnv) =====

	EmailDomains     SourcedStringSlice
	AllowedGroups    SourcedStringSlice
	WhitelistDomains SourcedStringSlice

	// ===== Routing Settings (merged, supports fromEnv) =====

	RedirectURL     SourcedValue
	ExtraJWTIssuers SourcedStringSlice

	// ===== Header Settings (merged, supports fromEnv) =====

	PassAccessToken         SourcedBool
	SetXAuthRequest         SourcedBool
	PassAuthorizationHeader SourcedBool

	// ===== Behavior Settings (merged, supports fromEnv) =====

	SkipProviderButton  SourcedBool
	SkipJWTBearerTokens SourcedBool

	// ===== Container Settings =====

	// ProxyImage is the oauth2-proxy container image (plain string, no fromEnv)
	// This is used by the webhook at pod creation time, not by oauth2-proxy at runtime
	ProxyImage     string
	ExtraArgs      []string                    // ConfigMap only, no fromEnv
	ProxyResources *corev1.ResourceRequirements // ConfigMap only

	// ===== Pod-Specific Settings (annotation-only, NO fromEnv support) =====
	// These are inherently per-pod and wouldn't make sense from env vars

	BlockDirectAccess bool
	ProtectedPort     string
	Upstream          SourcedValue               // supports fromEnv (not strictly pod-specific)
	UpstreamTLS       annotation.UpstreamTLSMode // "http", "https", "https-insecure"
	IgnorePaths       []string                   // pod-specific routing
	APIPaths          []string                   // pod-specific routing
	PingPath          string                     // pod-specific probe config
	ReadyPath         string                     // pod-specific probe config

	// ===== Secret Provider Class (CSI Driver) =====

	// SecretProviderClass is the name of a SecretProviderClass for CSI secrets driver
	// When set, a CSI volume is mounted at /etc/oauth2-proxy/conf.d/ and config values
	// can be read from files. File names map to annotation keys (e.g., "client-secret").
	//
	// Special handling for sensitive values:
	//   - client-secret -> uses --client-secret-file flag
	//   - cookie-secret -> uses --cookie-secret-file flag
	SecretProviderClass string

	// EnvSecret is the name of a Secret to use for env var injection
	// When set, fields with "fromEnv" source will generate env var entries
	// that read from this Secret using the annotation name as the key
	EnvSecret string
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
