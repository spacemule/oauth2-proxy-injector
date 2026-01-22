package mutation

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/spacemule/oauth2-proxy-injector/internal/config"
)

// InitContainerBuilder defines the interface for building init containers
// that set up network rules to block direct access to protected ports
type InitContainerBuilder interface {
	// Build creates an init container that configures iptables rules
	// to block direct access to the protected port(s), forcing traffic
	// through oauth2-proxy on localhost
	//
	// Returns nil if no init container is needed (feature disabled)
	Build(cfg *config.EffectiveConfig, portMapping PortMapping) *corev1.Container
}

// IPTablesInitContainerBuilder implements InitContainerBuilder for iptables-based port blocking
type IPTablesInitContainerBuilder struct {
	// initImage is the container image that provides iptables
	// Default: a minimal Alpine-based image with iptables installed
	initImage string
}

// NewIPTablesInitContainerBuilder creates a new IPTablesInitContainerBuilder
//
// TODO:
// 1. Accept initImage parameter (e.g., "alpine:latest" or custom image with iptables)
// 2. Return initialized builder
func NewIPTablesInitContainerBuilder(initImage string) *IPTablesInitContainerBuilder {
	panic("TODO: implement")
}

// Build creates an iptables init container if block-direct-access is enabled
//
// TODO:
// 1. Check if cfg.BlockDirectAccess is enabled (annotation-based opt-in)
// 2. If disabled, return nil (no init container needed)
// 3. Build iptables rules to:
//    a. Accept traffic from localhost (127.0.0.1) to the protected port
//    b. Drop all other traffic to the protected port
// 4. Create container spec with:
//    - Name: "oauth2-proxy-iptables-init"
//    - Image: from builder's initImage field
//    - Command: iptables rules as shell script
//    - SecurityContext: NET_ADMIN capability required
// 5. Return container spec
//
// Implementation notes:
// - iptables rules should be idempotent (handle re-runs gracefully)
// - Use INPUT chain to filter incoming connections
// - Example rule: iptables -A INPUT -p tcp --dport <port> ! -s 127.0.0.1 -j DROP
// - Consider using iptables-restore for atomic rule application
// - Must run as privileged or with NET_ADMIN capability
//
// Security considerations:
// - Only block the specific protected port, not all ports
// - Ensure localhost traffic is always allowed (health checks from app itself)
// - Consider IPv6 if needed (ip6tables)
func (b *IPTablesInitContainerBuilder) Build(cfg *config.EffectiveConfig, portMapping PortMapping) *corev1.Container {
	panic("TODO: implement")
}

// buildIPTablesScript generates the shell script that sets up iptables rules
//
// TODO:
// 1. Accept port number to protect
// 2. Generate shell script that:
//    a. Checks if iptables is available
//    b. Creates rules to accept localhost traffic to port
//    c. Creates rules to drop non-localhost traffic to port
//    d. Applies rules to INPUT chain
// 3. Return script as string
//
// Example output:
//   #!/bin/sh
//   set -e
//   # Allow localhost connections to protected port
//   iptables -A INPUT -p tcp --dport 8888 -s 127.0.0.1 -j ACCEPT
//   # Drop all other connections to protected port
//   iptables -A INPUT -p tcp --dport 8888 -j DROP
//   echo "iptables rules applied successfully"
//
// Advanced considerations:
// - Use iptables-save/restore for atomicity
// - Handle IPv6 with ip6tables if needed
// - Add logging rules for debugging (--log-prefix "oauth2-proxy-block: ")
func buildIPTablesScript(port int32) string {
	panic("TODO: implement")
}

// needsSecurityContext returns a SecurityContext with NET_ADMIN capability
//
// TODO:
// 1. Create SecurityContext struct
// 2. Add NET_ADMIN to capabilities.add
// 3. Optionally set privileged: true (more permissive, simpler but less secure)
// 4. Return SecurityContext pointer
//
// Notes:
// - NET_ADMIN capability is required to modify iptables rules
// - privileged=true gives full access but is a security concern
// - Prefer capabilities.add = [NET_ADMIN] over privileged mode when possible
// - Some clusters may have PodSecurityPolicies that restrict this
func needsSecurityContext() *corev1.SecurityContext {
	panic("TODO: implement")
}
