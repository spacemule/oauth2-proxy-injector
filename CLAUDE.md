# oauth2-proxy Mutating Admission Webhook

## Project Overview

I'm learning Go by building a Kubernetes mutating admission webhook that injects oauth2-proxy sidecars into pods. This is a LEARNING PROJECT - I implement the code myself to learn Go concepts.

## Current Status: IMPLEMENTATION IN PROGRESS

Completed:
- ✅ `internal/annotation/parser.go` - fully implemented
- ✅ `internal/mutation/patch.go` - fully implemented (fluent PatchBuilder)
- ✅ `internal/config/types.go` - fully implemented
- ✅ `internal/config/loader.go` - fully implemented
- ✅ `internal/mutation/sidecar.go` - CalculatePortMapping implemented

**Next step**: `internal/mutation/sidecar.go` - remaining functions (buildProbe, buildEnvVars, buildArgs, Build)

Remaining:
1. `internal/mutation/sidecar.go` - buildProbe, buildEnvVars, buildArgs, Build
2. `internal/mutation/mutator.go` - ties everything together
3. `internal/admission/handler.go` - HTTP/admission layer
4. `cmd/webhook/main.go` - wire it all up
5. Tests in `handler_test.go`

## Core Requirements

### Annotation-Based Configuration

Pods opt-in via annotations. Example:

```yaml
metadata:
  annotations:
    # === Core (required) ===
    spacemule.net/oauth2-proxy.enabled: "true"
    spacemule.net/oauth2-proxy.config: "plex-config"  # references a ConfigMap

    # === Port/Routing (annotation-only) ===
    spacemule.net/oauth2-proxy.protected-port: "http"  # port NAME to protect (default: "http")
    spacemule.net/oauth2-proxy.upstream-tls: "http"  # "http" (default), "https", or "https-insecure"
    spacemule.net/oauth2-proxy.upstream: "http://127.0.0.1:8080"  # override auto-calculated upstream
    spacemule.net/oauth2-proxy.ignore-paths: "/health,/metrics"  # paths to skip auth
    spacemule.net/oauth2-proxy.api-paths: "/api/"  # paths requiring JWT only (no login redirect)
    spacemule.net/oauth2-proxy.skip-jwt-bearer-tokens: "true"  # skip login when JWT provided

    # === ConfigMap Overrides (optional - override namespace defaults) ===
    # Identity overrides
    spacemule.net/oauth2-proxy.client-id: "my-service-client"  # different OAuth app
    spacemule.net/oauth2-proxy.client-secret-ref: "my-service-secrets:client-secret"
    spacemule.net/oauth2-proxy.pkce-enabled: "true"

    # Authorization overrides (stricter than namespace default)
    spacemule.net/oauth2-proxy.allowed-groups: "admin,devops"  # restrict to specific groups
    spacemule.net/oauth2-proxy.allowed-emails: "alice@example.com,bob@example.com"
    spacemule.net/oauth2-proxy.email-domains: "example.com"

    # Routing overrides
    spacemule.net/oauth2-proxy.redirect-url: "https://myservice.example.com/oauth2/callback"
    spacemule.net/oauth2-proxy.extra-jwt-issuers: "https://issuer.example.com=myservice-api"

    # Header overrides
    spacemule.net/oauth2-proxy.pass-access-token: "true"
    spacemule.net/oauth2-proxy.set-xauthrequest: "true"
    spacemule.net/oauth2-proxy.pass-authorization-header: "true"

    # Behavior overrides
    spacemule.net/oauth2-proxy.skip-provider-button: "true"

    # Provider overrides (rarely needed - usually namespace-wide)
    spacemule.net/oauth2-proxy.provider: "oidc"  # override provider type
    spacemule.net/oauth2-proxy.oidc-issuer-url: "https://other-realm.example.com"
    spacemule.net/oauth2-proxy.oidc-groups-claim: "roles"  # different claim name
    spacemule.net/oauth2-proxy.cookie-secure: "false"  # for dev/testing only

    # Container overrides
    spacemule.net/oauth2-proxy.proxy-image: "quay.io/oauth2-proxy/oauth2-proxy:v7.6.0"
```

### Port Protection Modes

The webhook supports two modes based on how `protected-port` is specified:

**Named Port (e.g., `protected-port: "http"`)** - Takeover mode:
- oauth2-proxy takes over the named port from the app container
- App container's port is removed
- Services using `targetPort: <name>` automatically route to the proxy
- Probes referencing the port name are rewritten to use numeric port
- Default: `"http"`

**Numbered Port (e.g., `protected-port: "8080"`)** - Service mutation mode:
- App container keeps all its ports unchanged
- oauth2-proxy sidecar gets a generic port name (`oauth2-proxy`)
- Probes are NOT rewritten (they still point at app container)
- Use with Service webhook to rewrite `targetPort` values
- Upstream is auto-calculated from the numbered port

**Upstream-only mode** (e.g., `upstream: "http://127.0.0.1:8080"`):
- Alternative to numbered port mode
- Explicitly specify the upstream URL instead of auto-calculating
- Useful for edge cases where port detection doesn't work

**Validation**: Either `protected-port` OR `upstream` must be set. If neither is provided, it's an error.

### ConfigMap-Based Proxy Settings

oauth2-proxy args come from a ConfigMap (namespaced or cluster-wide, your call on design). Things like:
- OIDC issuer URL
- Client ID / secret reference
- Allowed groups/emails
- Cookie settings
- Any other oauth2-proxy flags

### Upstream TLS Handling

Some pods (annoyingly) terminate TLS at the container level. Support via `upstream-tls` annotation:
- `http` - plain HTTP to upstream (default)
- `https` - HTTPS with certificate verification
- `https-insecure` - HTTPS without certificate verification (for self-signed certs)

## Technical Constraints

- Go 1.22+
- Use `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`
- No controller-runtime (this isn't an operator)
- Standard library where possible
- I run openSUSE MicroOS with Tumbleweed containers - don't suggest other distros

## Learning Goals

I want to practice these Go concepts (USE THEM WHERE RELEVANT):
- **Interfaces**: Define contracts for components (config loading, patch generation, etc.)
- **Methods**: Receiver functions on structs

## Project Structure Request

Scaffold this structure:

```
oauth2-proxy-webhook/
├── cmd/
│   └── webhook/
│       └── main.go              # entrypoint - TODO steps only
├── internal/
│   ├── admission/
│   │   ├── handler.go           # AdmissionReview handling - interfaces + function sigs
│   │   └── handler_test.go      # test file scaffold
│   ├── config/
│   │   ├── loader.go            # ConfigMap loading - interfaces + function sigs  
│   │   └── types.go             # config structs
│   ├── mutation/
│   │   ├── mutator.go           # pod mutation logic - interfaces + function sigs
│   │   ├── patch.go             # JSON patch generation
│   │   └── sidecar.go           # sidecar container building
│   └── annotation/
│       └── parser.go            # annotation parsing
├── deploy/
│   ├── webhook-deployment.yaml  # k8s manifests (scaffold with TODOs)
│   ├── webhook-service.yaml
│   ├── mutatingwebhook.yaml
│   └── example-configmap.yaml   # example oauth2-proxy config
├── Makefile                     # build, test, deploy targets
├── Dockerfile                   # multi-stage build
└── go.mod
```

## What I Want From You

1. Create the directory structure
2. In each `.go` file:
   - Write package declaration
   - Write imports (what I'll need)
   - Write interface definitions where appropriate
   - Write function/method signatures with params and return types
   - Write TODO comments explaining what each function should do, broken into steps
   - DO NOT write the implementation - leave function bodies empty or with `panic("TODO")`
3. In YAML files, scaffold with TODO comments explaining what's needed
4. Makefile with standard targets

## Example of What I Want in Code Files

```go
package mutation

import (
    corev1 "k8s.io/api/core/v1"
)

// Mutator defines the contract for pod mutation operations
type Mutator interface {
    // Mutate takes a pod and returns JSON patch operations
    Mutate(pod *corev1.Pod) ([]PatchOperation, error)
}

// PodMutator implements Mutator for oauth2-proxy sidecar injection
type PodMutator struct {
    configLoader ConfigLoader
}

// NewPodMutator creates a new PodMutator instance
// TODO:
// 1. Accept a ConfigLoader dependency
// 2. Return initialized PodMutator
func NewPodMutator(loader ConfigLoader) *PodMutator {
    panic("TODO: implement")
}

// Mutate inspects pod annotations and injects oauth2-proxy sidecar
// TODO:
// 1. Parse annotations from pod using annotation.Parser
// 2. If not enabled, return empty patch slice
// 3. Load proxy config from ConfigMap via configLoader
// 4. Identify protected ports (explicit list or all non-ignored)
// 5. Build sidecar container with proper upstreams
// 6. Handle upstream TLS mode (insecure/secure)
// 7. Generate JSON patch operations for:
//    - Adding sidecar container
//    - Adding volumes for secrets/certs if needed
// 8. Return patch operations
func (m *PodMutator) Mutate(pod *corev1.Pod) ([]PatchOperation, error) {
    panic("TODO: implement")
}
```

## Questions to Address in TODOs

- How should port remapping work? (proxy listens on X, forwards to localhost:original)
- Where do secrets (client secret, cookie secret) come from? (SecretRef in ConfigMap?)
- Should the webhook validate the referenced ConfigMap exists?
- How to handle pods that already have an oauth2-proxy container?

## Key Interfaces

These are the main contracts to implement:

- **`annotation.Parser`** - Parses pod annotations into a `Config` struct
- **`config.Loader`** - Loads `ProxyConfig` from Kubernetes ConfigMaps
- **`mutation.Mutator`** - Orchestrates the mutation (main business logic)
- **`mutation.SidecarBuilder`** - Builds the oauth2-proxy container spec
- **`mutation.PatchBuilder`** - Fluent builder for JSON Patch operations

## Design Decisions Made

- **Port takeover**: oauth2-proxy takes over the named port from the app container; Services using `targetPort: <name>` route to proxy automatically
- **Single port**: Only one port protected per pod (oauth2-proxy limitation - single `--http-address`)
- **Named ports required**: Protected ports must have a name for Service routing to work
- **Secrets**: Referenced via `SecretRef` in ConfigMap (e.g., `client-secret-ref: "oauth2-secrets:client-secret"`)
- **Double-injection prevention**: Check for `spacemule.net/oauth2-proxy.injected` annotation
- **Failure mode**: Mutation fails if ConfigMap doesn't exist (fail secure)
- **PKCE support**: Set `pkce-enabled: "true"` in ConfigMap to skip client secret requirement

## Configuration Architecture

### ConfigMap as Namespace Defaults, Annotations as Overrides

The design supports a "one ConfigMap per namespace, override per service" pattern:

```
┌─────────────────────────────────────────────────────────────────┐
│                    ConfigMap (Namespace-level)                   │
│  Base configuration shared across services in a namespace        │
│  - OIDC Issuer, Provider type (namespace-wide)                   │
│  - Default cookie settings                                       │
│  - Default allowed-groups, email-domains                         │
├─────────────────────────────────────────────────────────────────┤
│                    Pod Annotations (Per-service)                 │
│  Override specific settings for this service                     │
│  - Different client-id/secret (different OAuth app)              │
│  - Different redirect-url (service-specific callback)            │
│  - Stricter allowed-groups, allowed-emails                       │
│  - Service-specific extra-jwt-issuers                            │
└─────────────────────────────────────────────────────────────────┘
```

### Field Categories

| Category | ConfigMap Only | Both (CM + Annotation Override) |
|----------|---------------|--------------------------------|
| **Provider** | - | provider, oidc-issuer-url, oidc-groups-claim, scope |
| **Identity** | - | client-id, client-secret-ref, pkce-enabled |
| **Cookies** | - | cookie-secure, cookie-secret-ref, cookie-domains, cookie-name |
| **Authorization** | - | whitelist-domains, email-domains, allowed-groups, allowed-emails |
| **Routing** | - | redirect-url, extra-jwt-issuers |
| **Headers** | - | pass-access-token, set-xauthrequest, pass-authorization-header |
| **Behavior** | extra-args | skip-provider-button, proxy-image |

Annotation-only fields: protected-port, upstream-tls, ignore-paths, api-paths, skip-jwt-bearer-tokens, **upstream**

### Override Semantics

- **Pointer fields** (`*string`, `*bool`): `nil` = use ConfigMap value; non-nil = override
- **Slice fields with Set flag**: `Set=false` = use ConfigMap; `Set=true` = use annotation (even if empty)
- This allows explicitly setting "no groups allowed" vs "use default groups"

### Annotation-Only Mode

ConfigMaps are **optional**. You can configure everything via annotations:

```yaml
metadata:
  annotations:
    spacemule.net/oauth2-proxy.enabled: "true"
    # No config annotation = no ConfigMap required

    # Required fields (must be provided via annotation if no ConfigMap):
    spacemule.net/oauth2-proxy.provider: "oidc"
    spacemule.net/oauth2-proxy.oidc-issuer-url: "https://auth.example.com/realms/myrealm"
    spacemule.net/oauth2-proxy.client-id: "my-client"
    spacemule.net/oauth2-proxy.cookie-secret-ref: "my-secrets:cookie-secret"
    # Either client-secret-ref OR pkce-enabled required:
    spacemule.net/oauth2-proxy.pkce-enabled: "true"
```

Use cases:
- Simple single-service deployments
- Testing/development without shared ConfigMaps
- Services that need completely custom config

## TODO

- ✅ ~~Add annotations for oauth2-proxy access control options~~ (Done: types.go, parser.go updated)
- ✅ ~~Update `internal/config/loader.go` to parse new ConfigMap fields~~ (Done)
- ✅ ~~Add types: `EffectiveConfig`, `ConfigOverrides`, new fields in `ProxyConfig`~~ (Done)
- ✅ ~~Add annotation constants for overrides~~ (Done in parser.go)
- ✅ ~~Implement `internal/annotation/parser.go` Parse()~~ (Done)
- ✅ ~~Implement `internal/config/merge.go`~~ (Done)
- ✅ ~~Update `internal/mutation/sidecar.go` to use EffectiveConfig~~ (Done)
- ✅ ~~Update `internal/mutation/mutator.go` to integrate config merging~~ (Done)
- ✅ ~~Handle upstream override in sidecar.go buildArgs()~~ (Done)
- ✅ ~~Implement upstream-only mode validation in merge.go~~ (Done - requires either protected-port OR upstream)
- ✅ ~~Implement named vs numbered port behavior in mutator.go and sidecar.go~~ (Done)
- ✅ ~~Implement Service mutator~~ (Done - internal/service/*.go)

## Service Mutating Webhook

For multi-port scenarios (e.g., MediaMTX with HLS + WebRTC), the Service webhook rewrites `targetPort` values to route through oauth2-proxy.

### How It Works

```
┌─────────────────────────────────────────────────────────────────┐
│                         Pod                                      │
│  ┌─────────────────┐    ┌─────────────────┐                      │
│  │   App Container │    │  oauth2-proxy   │                      │
│  │   port: 8080    │◄───│  port: 4180     │◄── Service routes    │
│  │   port: 8554    │    │  upstream:8080  │    here via rewrite  │
│  └─────────────────┘    └─────────────────┘                      │
└─────────────────────────────────────────────────────────────────┘
```

### Service Annotations

```yaml
apiVersion: v1
kind: Service
metadata:
  annotations:
    # List ports to route through oauth2-proxy (comma-separated)
    spacemule.net/oauth2-proxy.rewrite-ports: "http,hls"
    # Or by number for unnamed ports:
    spacemule.net/oauth2-proxy.rewrite-ports: "8080,8554"
    # Optional: override proxy port (default: 4180)
    spacemule.net/oauth2-proxy.proxy-port: "4180"
spec:
  ports:
    - name: http
      port: 80
      targetPort: http    # Webhook rewrites to 4180
    - name: hls
      port: 8888
      targetPort: 8554    # Webhook rewrites to 4180
```

### MediaMTX Example

For MediaMTX with HLS on port 8888 and the main API on 8889:

**Pod annotations:**
```yaml
spacemule.net/oauth2-proxy.enabled: "true"
spacemule.net/oauth2-proxy.protected-port: "8888"  # Numbered port = service mode
```

**Service annotations:**
```yaml
spacemule.net/oauth2-proxy.rewrite-ports: "hls"  # Only rewrite HLS port
```

This way, HLS traffic goes through oauth2-proxy, while other ports remain direct.

### Files

- `internal/service/mutator.go` - Service mutation logic
- `internal/service/handler.go` - Admission webhook handler
- `deploy/mutatingwebhook.yaml` - Includes Service webhook config

### Endpoints

- `/mutate` or `/mutate-pod` - Pod mutation (existing)
- `/mutate-service` - Service mutation (new)

## Future Features

### iptables Init Container for Port Blocking

**Problem**: In service mode, the app container's ports are still accessible directly via pod IP, bypassing oauth2-proxy.

**Solution**: Add an init container that runs iptables rules to block direct connections to protected ports, only allowing traffic from oauth2-proxy (localhost).

**Implementation notes**:
- Init container needs `NET_ADMIN` capability
- Block incoming connections to protected port(s) except from 127.0.0.1
- Example rule: `iptables -A INPUT -p tcp --dport 8888 ! -s 127.0.0.1 -j DROP`
- **Health check consideration**: Kubelet health checks come from the node, not localhost. If probes target the protected port directly, they will be blocked. Options:
  - Use a separate health check port that isn't protected
  - Route health checks through oauth2-proxy (add to `ignore-paths`)
  - Allow traffic from the node IP (more complex, need to detect node IP)
- May need annotation to opt-in: `spacemule.net/oauth2-proxy.block-direct-access: "true"`