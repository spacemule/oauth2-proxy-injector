package annotation

import (
	"fmt"
	"strings"
)

// Annotation key constants - all under the spacemule.net domain
const (
	// AnnotationPrefix is the base prefix for all oauth2-proxy annotations
	AnnotationPrefix = "spacemule.net/oauth2-proxy."

	// KeyEnabled indicates whether oauth2-proxy injection is enabled for this pod
	// Value: "true" or "false"
	KeyEnabled = AnnotationPrefix + "enabled"

	// KeyConfig references the ConfigMap name containing oauth2-proxy configuration
	// Value: ConfigMap name (e.g., "plex-config")
	KeyConfig = AnnotationPrefix + "config"

	// KeyProtectedPort specifies which container port should be protected
	// Value: port name (e.g., "http")
	// default "http"
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
type Config struct {
	// Enabled indicates whether oauth2-proxy should be injected
	Enabled bool

	// ConfigMapName is the name of the ConfigMap containing oauth2-proxy settings
	ConfigMapName string

	// ProtectedPort is the name of the port that should be proxied
	ProtectedPort string

	// IgnorePaths is the list of paths that should NOT be proxied
	IgnorePaths []string

	// APIPaths is the list of paths that should not offer login and instead require a JWT
	APIPaths []string
	
	// Whether or not to skip login when bearer tokens are provided. Defaults to false (i.e. do not skip login)
	SkipJWTBearerTokens bool

	// UpstreamTLS is the TLS mode for upstream connections
	UpstreamTLS UpstreamTLSMode
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

func (p *AnnotationParser) Parse(annotations map[string]string) (*Config, error) {
	var (
		cm string
		port string = "http"
		ignorePaths []string
		apiPaths []string
		skipJWTBearerTokens bool = false
		upstreamTLS UpstreamTLSMode = UpstreamNoTLS
	)
	
	if annotations[KeyEnabled] != "true" {
		return &Config{Enabled: false}, nil		
	}

	cm, ok := annotations[KeyConfig]
	if !ok {
		return nil, fmt.Errorf("required annotation %s not found", KeyConfig)
	}
	
	port, ok = annotations[KeyProtectedPort]
	if ok {
		port = strings.TrimSpace(port)
	}

	ignorePathsStr, ok := annotations[KeyIgnorePaths]
	if ok {
		ignorePaths = parsePaths(ignorePathsStr)
	}

	apiPathsStr, ok := annotations[KeyAPIPaths]
	if ok {
		apiPaths = parsePaths(apiPathsStr)
	}
	
	bearer, ok := annotations[KeySkipJWTBearerTokens]
	if ok {
		if bearer != "true" && bearer != "false" {
			return nil, fmt.Errorf("invalid skip-jwt value: %q (must be 'true' or 'false')", bearer)
		}
		if bearer == "true" {
			skipJWTBearerTokens = true
		}
	}
	
	tls, ok := annotations[KeyUpstreamTLS]
	if ok {
		if tls != string(UpstreamNoTLS) && tls != string(UpstreamTLSInsecure) && tls != string(UpstreamTLSSecure) {
			return nil, fmt.Errorf("invalid upstream-tls value: %q (must be %s, %s, or %s)", tls, UpstreamNoTLS, UpstreamTLSInsecure, UpstreamTLSSecure)
		}
		upstreamTLS = UpstreamTLSMode(tls)
	}
	
	return &Config{
		Enabled: true,
		ConfigMapName: cm,
		ProtectedPort: port,
		IgnorePaths: ignorePaths,
		APIPaths: apiPaths,
		SkipJWTBearerTokens: skipJWTBearerTokens,
		UpstreamTLS: upstreamTLS,
	}, nil
}

func parsePaths(pathsStr string) ([]string) {
	var result []string
	if pathsStr == "" {
		return result
	}
	for _, path := range strings.Split(pathsStr, ",") {
		result = append(result, strings.TrimSpace(path))
	}
	return result
}