package config

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// ProxyConfig holds the oauth2-proxy configuration loaded from a ConfigMap
// This represents the settings that will be passed to the oauth2-proxy sidecar
type ProxyConfig struct {
	// Name is the ConfigMap name this was loaded from
	Name string

	// Namespace is where the ConfigMap lives
	Namespace string

	// Provider is the OAuth2 provider (e.g., "oidc", "google", "github")
	Provider string

	// OIDCIssuerURL is the OIDC provider's issuer URL
	// Example: "https://auth.example.com/realms/myrealm"
	OIDCIssuerURL string

	// ClientID is the OAuth2 client ID
	// This might come from the ConfigMap directly or reference a Secret
	ClientID string

	// ClientSecretRef references a Secret containing the client secret
	// Format: "secret-name" (key defaults to "client-secret")
	// or "secret-name:key-name" for custom key
	ClientSecretRef *SecretRef

	// CookieSecretRef references a Secret containing the cookie encryption secret
	// oauth2-proxy requires a 16, 24, or 32 byte secret for AES
	CookieSecretRef *SecretRef

	// CookieDomain sets the domain for oauth2-proxy cookies
	// Leave empty for automatic domain detection
	CookieDomains []string

	// CookieSecure determines if cookies should have the Secure flag
	// Default: true (required for HTTPS)
	CookieSecure bool

	// EmailDomains restricts access to specific email domains
	// Example: ["example.com", "corp.example.com"]
	// Use ["*"] to allow all domains
	EmailDomains []string

	// AllowedGroups restricts access to users in specific groups
	// Works with providers that support group claims
	AllowedGroups []string

	// SkipProviderButton skips the "Sign in with X" button and redirects directly
	SkipProviderButton bool

	// ExtraArgs contains any additional oauth2-proxy arguments
	// These are passed directly to the sidecar container
	// Example: ["--pass-access-token=true", "--set-xauthrequest=true"]
	ExtraArgs []string

	// ProxyImage is the oauth2-proxy container image to use
	// Default: "quay.io/oauth2-proxy/oauth2-proxy:latest"
	ProxyImage string

	// ProxyResources specifies resource requests/limits for the sidecar
	// Optional - uses oauth2-proxy defaults if not set
	ProxyResources *corev1.ResourceRequirements

	// PKCEEnabled sets whether or not to use PKCE to allow for config without a client_secret
	PKCEEnabled bool
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
const (
	// CMKeyProvider is the oauth2 provider type
	CMKeyProvider = "provider"

	// CMKeyOIDCIssuerURL is the OIDC issuer URL
	CMKeyOIDCIssuerURL = "oidc-issuer-url"

	// CMKeyClientID is the OAuth2 client ID
	CMKeyClientID = "client-id"

	// CMKeyClientSecretRef references the client secret
	// Format: "secret-name" or "secret-name:key"
	CMKeyClientSecretRef = "client-secret-ref"

	// CMKeyCookieSecretRef references the cookie secret
	CMKeyCookieSecretRef = "cookie-secret-ref"

	// CMKeyCookieDomain is the cookie domain
	CMKeyCookieDomains = "cookie-domains"

	// CMKeyCookieSecure is whether cookies require HTTPS
	CMKeyCookieSecure = "cookie-secure"

	// CMKeyEmailDomains is comma-separated allowed email domains
	CMKeyEmailDomains = "email-domains"

	// CMKeyAllowedGroups is comma-separated allowed groups
	CMKeyAllowedGroups = "allowed-groups"

	// CMKeySkipProviderButton skips the provider button page
	CMKeySkipProviderButton = "skip-provider-button"

	// CMKeyExtraArgs is newline-separated extra arguments
	CMKeyExtraArgs = "extra-args"

	// CMKeyProxyImage is the oauth2-proxy container image
	CMKeyProxyImage = "proxy-image"

	// CMKeyPKCEEnabled is whether or not the proxy is using PKCE without a client secret
	CMKeyPKCEEnabled = "pkce-enabled"
)

// DefaultProxyImage is the default oauth2-proxy container image
const DefaultProxyImage = "quay.io/oauth2-proxy/oauth2-proxy:v7.5.1"

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
