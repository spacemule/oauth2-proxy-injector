# iptables Init Container Feature - Implementation Guide

## Overview

This feature adds an init container that configures iptables rules to block direct access to protected ports, forcing all traffic through oauth2-proxy. This prevents bypassing authentication by connecting directly to the pod IP.

## Files to Modify

### 1. `internal/annotation/parser.go`
**What to add:**
- New constant: `KeyBlockDirectAccess = AnnotationPrefix + "block-direct-access"`
- New field in `Config` struct: `BlockDirectAccess bool`
- Parsing logic in `Parse()` method to read the annotation (similar to other bool fields)

**Learning focus:** Adding new annotation support, struct field expansion

### 2. `internal/config/types.go`
**What to add:**
- New field in `EffectiveConfig` struct: `BlockDirectAccess bool`
- Add to annotation-only settings section (around line 315)

**Learning focus:** Understanding config architecture (annotation-only vs ConfigMap-mergeable fields)

### 3. `internal/config/merge.go`
**What to add:**
- Copy `BlockDirectAccess` from annotation config to effective config
- Add line: `effective.BlockDirectAccess = annotationCfg.BlockDirectAccess`

**Learning focus:** Config merging logic

### 4. `internal/mutation/initcontainer.go` ⭐ **MAIN IMPLEMENTATION**
**What to implement:**
- `NewIPTablesInitContainerBuilder()` - constructor
- `Build()` - main logic:
  - Check `cfg.BlockDirectAccess`, return nil if false
  - Extract port number from `portMapping.ProxyPort`
  - Build shell script with `buildIPTablesScript()`
  - Create `corev1.Container` spec
  - Set SecurityContext with `needsSecurityContext()`
- `buildIPTablesScript()` - generate iptables commands
- `needsSecurityContext()` - create SecurityContext with NET_ADMIN capability

**Learning focus:**
- Building Kubernetes container specs
- Working with SecurityContext and capabilities
- Shell script generation
- Conditional resource creation (return nil when disabled)

### 5. `internal/mutation/mutator.go`
**What to add:**
- New field in `PodMutator` struct: `initContainerBuilder InitContainerBuilder`
- Update `NewPodMutator()` to accept the builder
- In `Mutate()` method, after building sidecar:
  - Call `initContainerBuilder.Build(effectiveConfig, portMapping)`
  - If not nil, add init container to patch operations
  - Use `PatchBuilder` to add to `/spec/initContainers` path

**Learning focus:**
- Dependency injection pattern
- JSON Patch operations for init containers
- Integrating new component into existing mutation flow

### 6. `cmd/webhook/main.go`
**What to add:**
- Create init container builder: `initBuilder := mutation.NewIPTablesInitContainerBuilder("alpine:latest")`
- Pass to `NewPodMutator()` constructor

**Learning focus:** Wiring up dependencies at application startup

### 7. `deploy/example-pod.yaml` (NEW FILE)
**What to create:**
Example pod manifest showing the feature in action:
```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    spacemule.net/oauth2-proxy.enabled: "true"
    spacemule.net/oauth2-proxy.protected-port: "8080"
    spacemule.net/oauth2-proxy.block-direct-access: "true"
    spacemule.net/oauth2-proxy.ignore-paths: "/ping,/ready"
```

## Implementation Order

1. **Start with types** (parser.go, types.go, merge.go)
   - Add annotation constant
   - Add struct fields
   - Wire through merge logic
   - This establishes the data flow

2. **Implement init container builder** (initcontainer.go)
   - Start with `needsSecurityContext()` (simplest)
   - Then `buildIPTablesScript()` (pure string generation)
   - Then `NewIPTablesInitContainerBuilder()` (simple constructor)
   - Finally `Build()` (ties it all together)

3. **Integrate into mutator** (mutator.go, main.go)
   - Add field and constructor param
   - Call builder in Mutate()
   - Add patch operation for init container

4. **Test and document** (example-pod.yaml)
   - Create example manifest
   - Test in cluster

## Key Design Decisions

### Health Check Strategy
**Decision:** Proxy health checks through oauth2-proxy

- Add `/ping` and `/ready` to `ignore-paths` automatically when `block-direct-access` is enabled
- oauth2-proxy's own health endpoints will respond
- Alternative (more complex): Allow node IP in iptables rules, but requires detecting node IP

### iptables Rules Approach
**Decision:** Simple INPUT chain rules

- Rule 1: Accept traffic from 127.0.0.1 to protected port
- Rule 2: Drop all other traffic to protected port
- Simpler than FORWARD chain manipulation
- Works for pod-to-pod and external traffic

### Container Image
**Decision:** Use Alpine with iptables

- Default: `alpine:latest` (small, includes iptables)
- Make configurable via constructor parameter
- Consider: Custom image with only iptables binary for minimal attack surface

### Capability vs Privileged
**Decision:** Use NET_ADMIN capability (not privileged)

- More secure than privileged mode
- Sufficient for iptables manipulation
- Some clusters may still restrict this via PSP/PSA

## Testing Checklist

- [ ] Annotation parsing works (true/false/1/0)
- [ ] Init container created when enabled
- [ ] Init container NOT created when disabled
- [ ] SecurityContext has NET_ADMIN capability
- [ ] iptables script has correct port number
- [ ] Health checks work (proxied through oauth2-proxy)
- [ ] Direct access to protected port is blocked
- [ ] Access through oauth2-proxy works
- [ ] Works with both named and numbered ports

## Questions for You to Consider

1. **IPv6 support:** Should we also add ip6tables rules? (Probably yes for completeness)

2. **Idempotency:** iptables `-A` appends rules on every restart. Should we use `-C` to check + `-A`, or use `-I` to insert, or use iptables-restore?

3. **Logging:** Should blocked connections be logged via `--log-prefix` for debugging?

4. **Health check automation:** Should the mutator automatically add `/ping,/ready` to ignore-paths when block-direct-access is enabled, or require manual annotation?

5. **Pod Security Policies:** How should we document the required PSP/PSA configuration for clusters that enforce security policies?

6. **Init image configuration:** Should this be:
   - Hardcoded default (alpine:latest)
   - Configurable via webhook startup flag
   - Configurable via annotation (per-pod override)
   - Configurable via ConfigMap

## Documentation to Update

- [ ] README.md - Add feature overview
- [ ] CLAUDE.md - Mark feature as implemented
- [ ] Example manifests in deploy/
- [ ] Webhook deployment RBAC (if needed)
