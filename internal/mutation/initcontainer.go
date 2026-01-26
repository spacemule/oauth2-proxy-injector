package mutation

import (
	"fmt"
	"strings"

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
func NewIPTablesInitContainerBuilder(initImage string) *IPTablesInitContainerBuilder {
	return &IPTablesInitContainerBuilder{
		initImage: initImage,
	}
}

// Build creates an iptables init container if block-direct-access is enabled
func (b *IPTablesInitContainerBuilder) Build(cfg *config.EffectiveConfig, portMapping PortMapping) *corev1.Container {
	if !cfg.BlockDirectAccess {
		return nil
	}
	return &corev1.Container{
		Name: "oauth2-proxy-iptables-init",
		Image: b.initImage,
		Command: []string{"/bin/sh", "-c", buildIPTablesScript([]int32{portMapping.ProxyPort})},
		SecurityContext: needsSecurityContext(),
	}	
}

// buildIPTablesScript generates the shell script that sets up iptables rules
func buildIPTablesScript(ports []int32) string {
	var script strings.Builder

	script.WriteString("#!/bin/sh\n")
	script.WriteString("set -e\n")
	for _, p := range ports {
		script.WriteString(fmt.Sprintf("iptables -A INPUT -p tcp --dport %d -s 127.0.0.1 -j ACCEPT\n", p))
		script.WriteString(fmt.Sprintf("iptables -A INPUT -p tcp --dport %d -j DROP\n", p))
	}
	
	return script.String()
}

// needsSecurityContext returns a SecurityContext with NET_ADMIN capability
func needsSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		Capabilities: &corev1.Capabilities{
			Add: []corev1.Capability{"NET_ADMIN"},
		},
	}
}
