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
func (m *ConfigMerger) Merge(base *ProxyConfig, overrides *annotation.Config) (*EffectiveConfig, error) {
	cfg := &EffectiveConfig{
		ConfigMapName:      base.Name,
		ConfigMapNamespace: base.Namespace,
		ProxyResources:     base.ProxyResources,
		ExtraArgs:          base.ExtraArgs,
		
	}

	cfg.Provider = mergeString(base.Provider, overrides.Overrides.Provider)
	cfg.OIDCIssuerURL = mergeString(base.OIDCIssuerURL, overrides.Overrides.OIDCIssuerURL)
	cfg.OIDCGroupsClaim = mergeString(base.OIDCGroupsClaim, overrides.Overrides.OIDCGroupsClaim)
	cfg.CookieSecure = mergeBool(base.CookieSecure, overrides.Overrides.CookieSecure)
	cfg.ProxyImage = mergeString(base.ProxyImage, overrides.Overrides.ProxyImage)
	cfg.Upstream = mergeString("", overrides.Overrides.Upstream)

	cfg.ClientID = mergeString(base.ClientID, overrides.Overrides.ClientID)
	cfg.Scope = mergeString(base.Scope, overrides.Overrides.Scope)
	cfg.PKCEEnabled = mergeBool(base.PKCEEnabled, overrides.Overrides.PKCEEnabled)
	cfg.RedirectURL = mergeString(base.RedirectURL, overrides.Overrides.RedirectURL)
	cfg.PassAccessToken = mergeBool(base.PassAccessToken, overrides.Overrides.PassAccessToken)
	cfg.SetXAuthRequest = mergeBool(base.SetXAuthRequest, overrides.Overrides.SetXAuthRequest)
	cfg.PassAuthorizationHeader = mergeBool(base.PassAuthorizationHeader, overrides.Overrides.PassAuthorizationHeader)
	cfg.SkipProviderButton = mergeBool(base.SkipProviderButton, overrides.Overrides.SkipProviderButton)
	cfg.CookieName = mergeString(base.CookieName, overrides.Overrides.CookieName)
	if v, err := mergeSecretRef(base.ClientSecretRef, overrides.Overrides.ClientSecretRef, "client-secret"); err != nil {
		return nil, err
	} else {
		cfg.ClientSecretRef = v
	}
	if v, err := mergeSecretRef(base.CookieSecretRef, overrides.Overrides.CookieSecretRef, "cookie-secret"); err != nil {
		return nil, err
	} else {
		cfg.CookieSecretRef = v
	}
	cfg.CookieDomains = mergeStringSlice(base.CookieDomains, overrides.Overrides.CookieDomains, overrides.Overrides.CookieDomainsSet)
	cfg.WhitelistDomains = mergeStringSlice(base.WhitelistDomains, overrides.Overrides.WhitelistDomains, overrides.Overrides.WhitelistDomainsSet)
	cfg.EmailDomains = mergeStringSlice(base.EmailDomains, overrides.Overrides.EmailDomains, overrides.Overrides.EmailDomainsSet)
	cfg.AllowedGroups = mergeStringSlice(base.AllowedGroups, overrides.Overrides.AllowedGroups, overrides.Overrides.AllowedGroupsSet)
	// cfg.AllowedEmails = mergeStringSlice(base.AllowedEmails, overrides.Overrides.AllowedEmails, overrides.Overrides.AllowedEmailsSet)
	cfg.ExtraJWTIssuers = mergeStringSlice(base.ExtraJWTIssuers, overrides.Overrides.ExtraJWTIssuers, overrides.Overrides.ExtraJWTIssuersSet)

	cfg.BlockDirectAccess = overrides.BlockDirectAccess
	cfg.ProtectedPort = overrides.ProtectedPort
	cfg.IgnorePaths = overrides.IgnorePaths
	cfg.APIPaths = overrides.APIPaths
	cfg.SkipJWTBearerTokens = overrides.SkipJWTBearerTokens
	cfg.UpstreamTLS = overrides.UpstreamTLS

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
func (cfg *EffectiveConfig) Validate() error {
	if cfg.Provider == "" {
		return fmt.Errorf("\nprovider unset")
	}
	if cfg.Provider == "oidc" && cfg.OIDCIssuerURL == "" {
		return fmt.Errorf("\nprovider type oidc requires oidc-issuer-url")
	}
	if cfg.ClientID == "" {
		return fmt.Errorf("\nclient-id unset")
	}
	if !cfg.PKCEEnabled && cfg.ClientSecretRef == nil {
		return fmt.Errorf("\npkce must be enabled or client-secret-ref provided")
	}
	if cfg.CookieSecretRef == nil {
		return fmt.Errorf("\ncookie-secret-ref unset")
	}
	if cfg.RedirectURL != "" {
		if _, err := url.Parse(cfg.RedirectURL); err != nil {
			return fmt.Errorf("\nredirect-url invalid")
		}
	}
	// if cfg.Upstream != "" {
	// 	if _, err := url.Parse(cfg.RedirectURL); err != nil {
	// 		return fmt.Errorf("\nupstream invalid")
	// 	}
	// }
	if cfg.ExtraJWTIssuers != nil {
		for _, v := range cfg.ExtraJWTIssuers {
			parts := strings.Split(v, "=")
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return fmt.Errorf("\ninvalid extra-jwt-issuer format")
			}
		}
	}
	if cfg.ProtectedPort == "" && cfg.Upstream == "" {
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
	builder.WriteString(fmt.Sprintf("provider=%s, ", cfg.Provider))
	if cfg.OIDCIssuerURL != "" {
		builder.WriteString(fmt.Sprintf("oidc-issuer-url=%s, ", cfg.OIDCIssuerURL))
	}
	builder.WriteString(fmt.Sprintf("client-id=%s, ", cfg.ClientID))
	if cfg.ClientSecretRef != nil {
		builder.WriteString(fmt.Sprintf("client-secret-ref=%s:%s, ", cfg.ClientSecretRef.Name, cfg.ClientSecretRef.Key))
	}
	if cfg.CookieSecretRef != nil {
		builder.WriteString(fmt.Sprintf("cookie-secret-ref=%s:%s, ", cfg.CookieSecretRef.Name, cfg.CookieSecretRef.Key))
	}
	builder.WriteString(fmt.Sprintf("protected-port=%s, ", cfg.ProtectedPort))
	builder.WriteString(fmt.Sprintf("allowed-groups=[%s], ", strings.Join(cfg.AllowedGroups, ",")))
	builder.WriteString(fmt.Sprintf("email-domains=[%s]", strings.Join(cfg.EmailDomains, ",")))
	if cfg.RedirectURL != "" {
		builder.WriteString(fmt.Sprintf(", redirect-url=%s", cfg.RedirectURL))
	}
	builder.WriteString("}")

	return builder.String()
}
