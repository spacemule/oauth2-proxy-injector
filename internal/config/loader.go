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

	provider, ok := data[CMKeyProvider]
	if ok {
		cfg.Provider = strings.TrimSpace(provider)
	} else {
		return nil, fmt.Errorf("configmap missing required key %s", CMKeyProvider)
	}

	issuer, ok := data[CMKeyOIDCIssuerURL]
	if ok {
		cfg.OIDCIssuerURL = strings.TrimSpace(issuer)
	} else if provider == "oidc" {
		return nil, fmt.Errorf("provider type oidc requires %s", CMKeyOIDCIssuerURL)
	}

	cliendId, ok := data[CMKeyClientID]
	if ok {
		cfg.ClientID = strings.TrimSpace(cliendId)
	} else {
		return nil, fmt.Errorf("configmap missing required key %s", CMKeyClientID)
	}

	pkce, ok := data[CMKeyPKCEEnabled]
	if ok {
		cfg.PKCEEnabled, err = parseBool(pkce, false)
		if err != nil {
			return nil, err
		}
	} else {
		cfg.PKCEEnabled = false
	}

	clientSecretRef, ok := data[CMKeyClientSecretRef]
	if ok {
		cfg.ClientSecretRef, err = parseSecretRef(clientSecretRef, "client-secret")
		if err != nil {
			return nil, err
		}
	} else if !cfg.PKCEEnabled {
		return nil, fmt.Errorf("configmap missing key %s when %s is false", CMKeyClientSecretRef, CMKeyPKCEEnabled)
	}

	cookieSecretRef, ok := data[CMKeyCookieSecretRef]
	if ok {
		cfg.CookieSecretRef, err = parseSecretRef(cookieSecretRef, "cookie-secret")
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("configmap missing key %s", CMKeyCookieSecretRef)
	}

	cookieDomains, ok := data[CMKeyCookieDomains]
	if ok {
		cfg.CookieDomains = splitAndTrim(cookieDomains, ",")
	} else {
		cfg.CookieDomains = []string{}
	}

	cookieSecure, ok := data[CMKeyCookieSecure]
	if ok {
		cfg.CookieSecure, err = parseBool(cookieSecure, true)
		if err != nil {
			return nil, err
		}
	} else {
		cfg.CookieSecure = true
	}

	skipProviderButton, ok := data[CMKeySkipProviderButton]
	if ok {
		cfg.SkipProviderButton, err = parseBool(skipProviderButton, false)
		if err != nil {
			return nil, err
		}
	} else {
		cfg.SkipProviderButton = false
	}

	emailDomains, ok := data[CMKeyEmailDomains]
	if ok {
		cfg.EmailDomains = splitAndTrim(emailDomains, ",")
	} else {
		cfg.EmailDomains = []string{}
	}

	allowedGroups, ok := data[CMKeyAllowedGroups]
	if ok {
		cfg.AllowedGroups = splitAndTrim(allowedGroups, ",")

	}

	extraArgs, ok := data[CMKeyExtraArgs]
	if ok {
		cfg.ExtraArgs = splitAndTrim(extraArgs, "\n")

	}

	proxyImage, ok := data[CMKeyProxyImage]
	if ok {
		cfg.ProxyImage = strings.TrimSpace(proxyImage)
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
