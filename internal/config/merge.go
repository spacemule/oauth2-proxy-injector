package config

import (
	"fmt"
	"github.com/spacemule/oauth2-proxy-injector/internal/annotation"
	"net/url"
	"strings"
)

// Merger defines the interface for merging ConfigMap settings with annotation overrides
type Merger interface {
	// Merge combines a ProxyConfig (from ConfigMap) with annotation overrides
	// to produce the final EffectiveConfig used by the sidecar builder.
	//
	// The merge follows these rules:
	// - ConfigMap-only fields are copied directly (provider, oidc-issuer-url, etc.)
	// - Overridable fields use annotation value if set, otherwise ConfigMap value
	// - Annotation-only fields come only from the annotation Config
	// - Validation is performed on the merged result
	Merge(base *ProxyConfig, overrides *annotation.Config) (*EffectiveConfig, error)
}

// ConfigMerger implements Merger
type ConfigMerger struct{}

// NewMerger creates a new ConfigMerger
func NewMerger() *ConfigMerger {
	return &ConfigMerger{}
}

// Merge combines base ConfigMap settings with per-pod annotation overrides
//
// For fields that support ValueSource (file, fromEnv, literal):
//   - If annotation has ValueSource set, use its type and value
//   - If annotation is not set, use ConfigMap value with ValueSourceLiteral
//
// TODO:
// 1. For each SourcedValue field, call mergeSourcedValue helper
// 2. mergeSourcedValue should:
//    a. If override.IsSet() -> return SourcedValue{Value: override.Value, Source: override.Type}
//    b. Else -> return SourcedValue{Value: base, Source: ValueSourceLiteral}
// 3. For SourcedSecretRef fields:
//    a. If source is ValueSourceLiteral -> parse as SecretRef
//    b. If source is ValueSourceFile or ValueSourceEnv -> set source type, leave Ref nil
func (m *ConfigMerger) Merge(base *ProxyConfig, overrides *annotation.Config) (*EffectiveConfig, error) {
	cfg := &EffectiveConfig{
		ConfigMapName:      base.Name,
		ConfigMapNamespace: base.Namespace,
		ProxyResources:     base.ProxyResources,
		ExtraArgs:          base.ExtraArgs,
	}

	// Provider settings with SourcedValue support
	cfg.Provider = mergeSourcedValue(base.Provider, overrides.Overrides.Provider)
	cfg.OIDCIssuerURL = mergeSourcedValue(base.OIDCIssuerURL, overrides.Overrides.OIDCIssuerURL)
	cfg.OIDCGroupsClaim = mergeString(base.OIDCGroupsClaim, overrides.Overrides.OIDCGroupsClaim)
	cfg.Scope = mergeSourcedValue(base.Scope, overrides.Overrides.Scope)
	cfg.ValidateURL = mergeSourcedValue(base.ValidateURL, overrides.Overrides.ValidateURL)

	// Identity settings
	cfg.ClientID = mergeSourcedValue(base.ClientID, overrides.Overrides.ClientID)
	cfg.PKCEEnabled = mergeBool(base.PKCEEnabled, overrides.Overrides.PKCEEnabled)
	cfg.CodeChallengeMethod = mergeString(base.CodeChallengeMethod, overrides.Overrides.CodeChallengeMethod)

	// Client secret with SourcedSecretRef
	if v, err := mergeSourcedSecretRef(base.ClientSecretRef, overrides.Overrides.ClientSecretRef, "client-secret"); err != nil {
		return nil, err
	} else {
		cfg.ClientSecret = v
	}

	// Cookie secret with SourcedSecretRef
	if v, err := mergeSourcedSecretRef(base.CookieSecretRef, overrides.Overrides.CookieSecretRef, "cookie-secret"); err != nil {
		return nil, err
	} else {
		cfg.CookieSecret = v
	}

	// Cookie settings
	cfg.CookieSecure = mergeBool(base.CookieSecure, overrides.Overrides.CookieSecure)
	cfg.CookieName = mergeString(base.CookieName, overrides.Overrides.CookieName)
	cfg.CookieDomains = mergeStringSlice(base.CookieDomains, overrides.Overrides.CookieDomains, overrides.Overrides.CookieDomainsSet)

	// Container settings
	cfg.ProxyImage = mergeString(base.ProxyImage, overrides.Overrides.ProxyImage)

	// Routing settings with SourcedValue support
	cfg.RedirectURL = mergeSourcedValue(base.RedirectURL, overrides.Overrides.RedirectURL)
	cfg.Upstream = mergeSourcedValue("", overrides.Overrides.Upstream)

	// Other settings (no ValueSource support)
	cfg.PassAccessToken = mergeBool(base.PassAccessToken, overrides.Overrides.PassAccessToken)
	cfg.SetXAuthRequest = mergeBool(base.SetXAuthRequest, overrides.Overrides.SetXAuthRequest)
	cfg.PassAuthorizationHeader = mergeBool(base.PassAuthorizationHeader, overrides.Overrides.PassAuthorizationHeader)
	cfg.SkipProviderButton = mergeBool(base.SkipProviderButton, overrides.Overrides.SkipProviderButton)
	cfg.WhitelistDomains = mergeStringSlice(base.WhitelistDomains, overrides.Overrides.WhitelistDomains, overrides.Overrides.WhitelistDomainsSet)
	cfg.EmailDomains = mergeStringSlice(base.EmailDomains, overrides.Overrides.EmailDomains, overrides.Overrides.EmailDomainsSet)
	cfg.AllowedGroups = mergeStringSlice(base.AllowedGroups, overrides.Overrides.AllowedGroups, overrides.Overrides.AllowedGroupsSet)
	// cfg.AllowedEmails = mergeStringSlice(base.AllowedEmails, overrides.Overrides.AllowedEmails, overrides.Overrides.AllowedEmailsSet)
	cfg.ExtraJWTIssuers = mergeStringSlice(base.ExtraJWTIssuers, overrides.Overrides.ExtraJWTIssuers, overrides.Overrides.ExtraJWTIssuersSet)

	// Annotation-only settings
	cfg.BlockDirectAccess = overrides.BlockDirectAccess
	cfg.ProtectedPort = overrides.ProtectedPort
	cfg.IgnorePaths = overrides.IgnorePaths
	cfg.APIPaths = overrides.APIPaths
	cfg.SkipJWTBearerTokens = overrides.SkipJWTBearerTokens
	cfg.UpstreamTLS = overrides.UpstreamTLS
	cfg.PingPath = overrides.PingPath
	cfg.ReadyPath = overrides.ReadyPath
	cfg.SecretProviderClass = overrides.SecretProviderClass

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// mergeString returns override if non-nil, otherwise base
func mergeString(base string, override *string) string {
	if override != nil {
		return *override
	}
	return base
}

// mergeSourcedValue merges a base string value with a ValueSource override
//
// Returns a SourcedValue with the resolved value and source type:
//   - If override.IsSet() -> use override's Type and Value
//   - Else -> use base value with ValueSourceLiteral type
//
// TODO:
// 1. Check if override.IsSet()
// 2. If set:
//    a. Return SourcedValue{Value: override.Value, Source: override.Type}
//    b. Note: for file/env, Value will be empty (that's fine)
// 3. If not set:
//    a. Return SourcedValue{Value: base, Source: ValueSourceLiteral}
func mergeSourcedValue(base string, override annotation.ValueSource) SourcedValue {
	panic("TODO: implement")
}

// mergeSourcedSecretRef merges a base SecretRef with a ValueSource override
//
// Returns a SourcedSecretRef and any parsing error.
//
// When override is:
//   - Not set -> return SourcedSecretRef{Ref: base, Source: ValueSourceLiteral}
//   - ValueSourceLiteral -> parse override.Value as SecretRef
//   - ValueSourceFile -> return SourcedSecretRef{Ref: nil, Source: ValueSourceFile}
//   - ValueSourceEnv -> return SourcedSecretRef{Ref: nil, Source: ValueSourceEnv}
//
// TODO:
// 1. Check if override.IsSet()
// 2. If not set:
//    a. If base != nil -> return SourcedSecretRef{Ref: base, Source: ValueSourceLiteral}
//    b. Else -> return SourcedSecretRef{} (zero value, neither set)
// 3. If set:
//    a. Switch on override.Type:
//       - ValueSourceLiteral: parse override.Value as SecretRef using parseSecretRef
//       - ValueSourceFile: return SourcedSecretRef{Source: ValueSourceFile}
//       - ValueSourceEnv: return SourcedSecretRef{Source: ValueSourceEnv}
func mergeSourcedSecretRef(base *SecretRef, override annotation.ValueSource, defaultKey string) (SourcedSecretRef, error) {
	panic("TODO: implement")
}

// mergeBool returns override if non-nil, otherwise base
func mergeBool(base bool, override *bool) bool {
	if override != nil {
		return *override
	}
	return base
}

// mergeStringSlice returns override if set flag is true, otherwise base
func mergeStringSlice(base []string, override []string, overrideSet bool) []string {
	if overrideSet {
		return override
	}
	return base
}

// mergeSecretRef parses and returns override if non-nil, otherwise base
func mergeSecretRef(base *SecretRef, override *string, defaultKey string) (*SecretRef, error) {
	if override == nil {
		return base, nil
	}
	return parseSecretRef(*override, defaultKey)
}

// Validate checks that the EffectiveConfig is valid and complete
//
// Validation rules for SourcedValue fields:
//   - If source is ValueSourceEnv or ValueSourceFile, the value field is ignored
//     (oauth2-proxy will read from env or file at runtime)
//   - If source is ValueSourceLiteral, the value must be valid
//
// When SecretProviderClass is set, secret-related validations are relaxed
// because secrets will come from CSI-mounted files at runtime rather than
// from Kubernetes Secret references.
//
// TODO: Update validation to check source types:
// 1. For provider: skip "provider unset" check if source is fromEnv
// 2. For oidc-issuer-url: skip check if source is fromEnv
// 3. For client-id: skip "client-id unset" check if source is fromEnv
// 4. For secrets: skip validation if source is file or fromEnv
// 5. For redirect-url: only validate URL format if source is literal and value is set
// 6. For upstream: only check if ProtectedPort is also empty AND source is not fromEnv
func (cfg *EffectiveConfig) Validate() error {
	// Provider validation - skip if coming from env
	if cfg.Provider.IsLiteral() && cfg.Provider.Value == "" {
		return fmt.Errorf("\nprovider unset")
	}
	// OIDC issuer validation - only check if provider is literal "oidc" and issuer source is literal
	if cfg.Provider.IsLiteral() && cfg.Provider.Value == "oidc" {
		if cfg.OIDCIssuerURL.IsLiteral() && cfg.OIDCIssuerURL.Value == "" {
			return fmt.Errorf("\nprovider type oidc requires oidc-issuer-url")
		}
	}
	// Client ID validation - skip if coming from env
	if cfg.ClientID.IsLiteral() && cfg.ClientID.Value == "" {
		return fmt.Errorf("\nclient-id unset")
	}

	// Secret validations - skip when using SecretProviderClass or non-literal sources
	if cfg.SecretProviderClass == "" {
		// Client secret: required unless PKCE enabled OR source is file/env
		if !cfg.PKCEEnabled && cfg.ClientSecret.Ref == nil && cfg.ClientSecret.IsLiteral() {
			return fmt.Errorf("\npkce must be enabled or client-secret-ref provided")
		}

		// Cookie secret: required unless source is file/env
		if cfg.CookieSecret.Ref == nil && cfg.CookieSecret.IsLiteral() {
			return fmt.Errorf("\ncookie-secret-ref unset")
		}
	}

	// URL validation - only validate if literal and non-empty
	if cfg.RedirectURL.IsLiteral() && cfg.RedirectURL.Value != "" {
		if _, err := url.Parse(cfg.RedirectURL.Value); err != nil {
			return fmt.Errorf("\nredirect-url invalid")
		}
	}

	if cfg.ExtraJWTIssuers != nil {
		for _, v := range cfg.ExtraJWTIssuers {
			parts := strings.Split(v, "=")
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return fmt.Errorf("\ninvalid extra-jwt-issuer format")
			}
		}
	}

	// Port/upstream validation - need at least one way to determine where to proxy
	// Skip if upstream source is env (oauth2-proxy will read it)
	if cfg.ProtectedPort == "" && cfg.Upstream.Value == "" && !cfg.Upstream.IsFromEnv() {
		return fmt.Errorf("\nprotected-port or upstream must be set")
	}

	if cfg.UpstreamTLS != annotation.UpstreamNoTLS && cfg.UpstreamTLS != annotation.UpstreamTLSSecure && cfg.UpstreamTLS != annotation.UpstreamTLSInsecure {
		return fmt.Errorf("\nupstream-tls invalid")
	}

	return nil
}

// String returns a human-readable summary of the config for logging
func (cfg *EffectiveConfig) String() string {
	var builder strings.Builder

	builder.WriteString("EffectiveConfig{")
	builder.WriteString(fmt.Sprintf("configmap=%s/%s, ", cfg.ConfigMapName, cfg.ConfigMapNamespace))
	builder.WriteString(fmt.Sprintf("provider=%s, ", cfg.Provider.Value))
	if cfg.OIDCIssuerURL.Value != "" {
		builder.WriteString(fmt.Sprintf("oidc-issuer-url=%s, ", cfg.OIDCIssuerURL.Value))
	}
	builder.WriteString(fmt.Sprintf("client-id=%s, ", cfg.ClientID.Value))
	if cfg.ClientSecret.Ref != nil {
		builder.WriteString(fmt.Sprintf("client-secret-ref=%s:%s, ", cfg.ClientSecret.Ref.Name, cfg.ClientSecret.Ref.Key))
	}
	if cfg.CookieSecret.Ref != nil {
		builder.WriteString(fmt.Sprintf("cookie-secret-ref=%s:%s, ", cfg.CookieSecret.Ref.Name, cfg.CookieSecret.Ref.Key))
	}
	builder.WriteString(fmt.Sprintf("protected-port=%s, ", cfg.ProtectedPort))
	builder.WriteString(fmt.Sprintf("allowed-groups=[%s], ", strings.Join(cfg.AllowedGroups, ",")))
	builder.WriteString(fmt.Sprintf("email-domains=[%s]", strings.Join(cfg.EmailDomains, ",")))
	if cfg.RedirectURL.Value != "" {
		builder.WriteString(fmt.Sprintf(", redirect-url=%s", cfg.RedirectURL.Value))
	}
	builder.WriteString("}")

	return builder.String()
}
