package config

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Loader defines the interface for loading oauth2-proxy configuration
// This interface allows for easy testing with mock implementations
type Loader interface {
	// Load retrieves and parses a ProxyConfig from a ConfigMap
	// The namespace parameter determines where to look for the ConfigMap
	Load(ctx context.Context, name, namespace string) (*ProxyConfig, error)
}

// ConfigMapLoader implements Loader using Kubernetes ConfigMaps
type ConfigMapLoader struct {
	// client is the Kubernetes clientset for API calls
	client kubernetes.Interface

	// defaultNamespace is used when namespace is empty
	// Typically set to the webhook's own namespace
	defaultNamespace string
}

// NewLoader creates a new ConfigMapLoader
func NewLoader(client kubernetes.Interface, defaultNamespace string) *ConfigMapLoader {
	return &ConfigMapLoader{
		client: client,
		defaultNamespace: defaultNamespace,
	}
}

// Load retrieves a ConfigMap and parses it into a ProxyConfig
// DESIGN QUESTION (address in implementation):
// Should this validate that referenced Secrets exist?
// Pro: Fail fast with clear error
// Con: Additional API calls, might not have permission
// Suggestion: Add optional validation, default off
func (l *ConfigMapLoader) Load(ctx context.Context, name, namespace string) (*ProxyConfig, error) {
	n := namespace
	if namespace == "" {
		n = l.defaultNamespace
	}
	cm, err := l.client.CoreV1().ConfigMaps(n).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	
	cfg, err :=parseConfigMap(cm.Data, name, n)
	if err != nil {
		return nil, err
	}

	return cfg, nil	
}

// parseConfigMap converts ConfigMap data to ProxyConfig
func parseConfigMap(data map[string]string, name, namespace string) (*ProxyConfig, error) {
	cfg := &ProxyConfig{
		Name: name,
		Namespace: namespace,
	}
	var err error

	if v, ok := data[CMKeyProvider]; ok {
		cfg.Provider = strings.TrimSpace(v)
	} else {
		return nil, fmt.Errorf("configmap missing required key %s", CMKeyProvider)
	}

	if v, ok := data[CMKeyOIDCIssuerURL]; ok {
		cfg.OIDCIssuerURL = strings.TrimSpace(v)
	} else if cfg.Provider == "oidc" {
		return nil, fmt.Errorf("provider type oidc requires %s", CMKeyOIDCIssuerURL)
	}

	if v, ok := data[CMKeyClientID]; ok {
		cfg.ClientID = strings.TrimSpace(v)
	} else {
		return nil, fmt.Errorf("configmap missing required key %s", CMKeyClientID)
	}

	if v, ok := data[CMKeyPKCEEnabled]; ok {
		cfg.PKCEEnabled, err = parseBool(v, false)
		if err != nil {
			return nil, err
		}
	} else {
		cfg.PKCEEnabled = false
	}

	if v, ok := data[CMKeyClientSecretRef]; ok {
		cfg.ClientSecretRef, err = parseSecretRef(v, "client-secret")
		if err != nil {
			return nil, err
		}
	} else if !cfg.PKCEEnabled {
		return nil, fmt.Errorf("configmap missing key %s when %s is false", CMKeyClientSecretRef, CMKeyPKCEEnabled)
	}

	if v, ok := data[CMKeyCookieSecretRef]; ok {
		cfg.CookieSecretRef, err = parseSecretRef(v, "cookie-secret")
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("configmap missing key %s", CMKeyCookieSecretRef)
	}

	if v, ok := data[CMKeyCookieDomains]; ok {
		cfg.CookieDomains = splitAndTrim(v, ",")
	} else {
		cfg.CookieDomains = []string{}
	}

	if v, ok := data[CMKeyCookieSecure]; ok {
		cfg.CookieSecure, err = parseBool(v, true)
		if err != nil {
			return nil, err
		}
	} else {
		cfg.CookieSecure = true
	}

	if v, ok := data[CMKeySkipProviderButton]; ok {
		cfg.SkipProviderButton, err = parseBool(v, false)
		if err != nil {
			return nil, err
		}
	} else {
		cfg.SkipProviderButton = false
	}

	if v, ok := data[CMKeyEmailDomains]; ok {
		cfg.EmailDomains = splitAndTrim(v, ",")
	} else {
		cfg.EmailDomains = []string{}
	}

	if v, ok := data[CMKeyAllowedGroups]; ok {
		cfg.AllowedGroups = splitAndTrim(v, ",")
	}

	if v, ok := data[CMKeyExtraArgs]; ok {
		cfg.ExtraArgs = splitAndTrim(v, "\n")
	}

	if v, ok := data[CMKeyProxyImage]; ok {
		cfg.ProxyImage = strings.TrimSpace(v)
	}

	if v, ok := data[CMKeyOIDCGroupsClaim]; ok {
		cfg.OIDCGroupsClaim = strings.TrimSpace(v)
	} else {
		cfg.OIDCGroupsClaim = "groups" // default
	}

	if v, ok := data[CMKeyScope]; ok {
		cfg.Scope = strings.TrimSpace(v)
	}

	if v, ok := data[CMKeyCookieName]; ok {
		cfg.CookieName = strings.TrimSpace(v)
	}

	// if v, ok := data[CMKeyAllowedEmails]; ok {
	// 	cfg.AllowedEmails = splitAndTrim(v, ",")
	// }

	if v, ok := data[CMKeyWhitelistDomains]; ok {
		cfg.WhitelistDomains = splitAndTrim(v, ",")
	}

	if v, ok := data[CMKeyRedirectURL]; ok {
		cfg.RedirectURL = strings.TrimSpace(v)
	}

	if v, ok := data[CMKeyExtraJWTIssuers]; ok {
		cfg.ExtraJWTIssuers = splitAndTrim(v, ",")
	}

	if v, ok := data[CMKeyPassAccessToken]; ok {
		cfg.PassAccessToken, err = parseBool(v, false)
		if err != nil {
			return nil, err
		}
	}

	if v, ok := data[CMKeySetXAuthRequest]; ok {
		cfg.SetXAuthRequest, err = parseBool(v, false)
		if err != nil {
			return nil, err
		}
	}

	if v, ok := data[CMKeyPassAuthorizationHeader]; ok {
		cfg.PassAuthorizationHeader, err = parseBool(v, false)
		if err != nil {
			return nil, err
		}
	}

	
	return cfg, nil
}

// parseSecretRef parses a secret reference string
// Formats:
//   - "secret-name" -> SecretRef{Name: "secret-name", Key: defaultKey}
//   - "secret-name:custom-key" -> SecretRef{Name: "secret-name", Key: "custom-key"}
func parseSecretRef(ref string, defaultKey string) (*SecretRef, error) {
	ret := &SecretRef{}
	if ref == "" {
		return nil, nil
	}
	s := strings.SplitN(ref, ":", 2)

	ret.Name = s[0]
	if len(s) == 2 {
		if s[1] != "" {
			ret.Key = s[1]
		} else {
			return nil, fmt.Errorf("secretRef %s does not match expected format", s)
		}
	} else {
		ret.Key = defaultKey
	}
	return ret, nil
}

// parseBool parses a boolean string with a default value
func parseBool(value string, defaultValue bool) (bool, error) {
	switch strings.ToLower(value) {
	case "":
		return defaultValue, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "1":
		return true, nil
	case "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", value)
	}
}

// splitAndTrim splits a string by separator and trims whitespace from each element
func splitAndTrim(s, sep string) []string {
	ret := []string{}
	
	splits := strings.Split(s, sep)
	for _, split := range splits {
		if trimmed := strings.TrimSpace(split); trimmed != "" {
			ret = append(ret, trimmed)
		} 
	}

	return ret
}

// Suppress unused import errors during scaffolding
var (
	_ = context.Background
	_ = strings.Split
	_ = metav1.GetOptions{}
	_ kubernetes.Interface
)
