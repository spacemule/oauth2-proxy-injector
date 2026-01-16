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
    spacemule.net/oauth2-proxy.enabled: "true"
    spacemule.net/oauth2-proxy.config: "plex-config"  # references a ConfigMap
    spacemule.net/oauth2-proxy.protected-port: "http"  # port NAME to protect (default: "http")
    spacemule.net/oauth2-proxy.upstream-tls: "http"  # "http" (default), "https", or "https-insecure"
    spacemule.net/oauth2-proxy.ignore-paths: "/health,/metrics"  # paths to skip auth
    spacemule.net/oauth2-proxy.api-paths: "/api/"  # paths requiring JWT only (no login redirect)
    spacemule.net/oauth2-proxy.skip-jwt-bearer-tokens: "true"  # skip login when JWT provided
```

### Single Port Protection

The webhook protects a single **named** port per pod:
- Protected port must have a name (e.g., `name: http`)
- oauth2-proxy takes over that named port
- Services using `targetPort: <name>` automatically route to the proxy
- Default protected port name is "http"

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

## TODO

- Add annotations for oauth2-proxy access control options (allowed-groups, allowed-emails, etc.) - these need to be per-service rather than per-ConfigMap

## Future Projects

- **Service Mutating Webhook**: Create a separate webhook that mutates Services to redirect traffic through oauth2-proxy. Currently, this webhook requires protected ports to be **named** so that Services using `targetPort: <name>` automatically route to the proxy (since the webhook moves the named port from the app container to the sidecar). A Service webhook would handle unnamed ports by rewriting `targetPort` values directly.