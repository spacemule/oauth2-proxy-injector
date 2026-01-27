# oauth2-proxy-injector

More documentation to follow.

I used Claude to teach me go. Claude scaffolded this project, but all the code was human-written.

See Claude.md and the deploy directory for an idea of how to configure.

The mutating webhook adds a sidecar running oauth2-proxy targeting the named port (the service must also target the port by name at this point) in properly annotated pods. It adds easy authentication to services without having to edit existing helm-charts or manually add sidecars to manifests.

Configuration is done in via configmap for non-sensitive values that are stable across services and via annotation for service-specific configurations.

## Pod Annotations

### Core Annotations

| Annotation | Required | Default | Description |
|------------|----------|---------|-------------|
| `spacemule.net/oauth2-proxy.enabled` | Yes | - | Set to `"true"` to enable injection |
| `spacemule.net/oauth2-proxy.config` | No | webhook default | ConfigMap name containing oauth2-proxy settings |

### Port/Routing Annotations (Annotation-Only)

| Annotation | Required | Default | Description |
|------------|----------|---------|-------------|
| `spacemule.net/oauth2-proxy.protected-port` | No* | `"http"` | Port to protect. Named port (e.g., `"http"`) = takeover mode. Numbered port (e.g., `"8080"`) = service mode |
| `spacemule.net/oauth2-proxy.upstream` | No* | - | Explicit upstream URL (e.g., `"http://127.0.0.1:8080"`). Alternative to `protected-port` |
| `spacemule.net/oauth2-proxy.upstream-tls` | No | `"http"` | TLS mode for upstream: `"http"`, `"https"`, or `"https-insecure"` |
| `spacemule.net/oauth2-proxy.ignore-paths` | No | - | Comma-separated paths to skip auth (regex). Format: `path`, `method=path`, or `method!=path` |
| `spacemule.net/oauth2-proxy.api-paths` | No | - | Comma-separated paths requiring JWT only (no login redirect) |
| `spacemule.net/oauth2-proxy.skip-jwt-bearer-tokens` | No | `"false"` | Skip login when valid JWT bearer token is provided |
| `spacemule.net/oauth2-proxy.block-direct-access` | No | `"false"` | Block direct access to protected port via iptables (requires `NET_ADMIN` capability) |
| `spacemule.net/oauth2-proxy.ping-path` | No | `"/ping"` | Custom path for oauth2-proxy health check endpoint (use if conflicts with app) |
| `spacemule.net/oauth2-proxy.ready-path` | No | `"/ready"` | Custom path for oauth2-proxy ready endpoint (use if conflicts with app) |

*Either `protected-port` or `upstream` must be set.

### Identity Override Annotations

| Annotation | Default | Description |
|------------|---------|-------------|
| `spacemule.net/oauth2-proxy.client-id` | ConfigMap | OAuth2 client ID |
| `spacemule.net/oauth2-proxy.client-secret-ref` | ConfigMap | Secret reference for client secret (`"secret-name"` or `"secret-name:key"`) |
| `spacemule.net/oauth2-proxy.cookie-secret-ref` | ConfigMap | Secret reference for cookie secret |
| `spacemule.net/oauth2-proxy.scope` | ConfigMap | OAuth scopes to request |
| `spacemule.net/oauth2-proxy.pkce-enabled` | ConfigMap | Enable PKCE flow (`"true"` or `"false"`) |

### Authorization Override Annotations

| Annotation | Default | Description |
|------------|---------|-------------|
| `spacemule.net/oauth2-proxy.email-domains` | ConfigMap | Comma-separated allowed email domains. Use `"*"` for all |
| `spacemule.net/oauth2-proxy.allowed-groups` | ConfigMap | Comma-separated allowed groups |
| `spacemule.net/oauth2-proxy.whitelist-domains` | ConfigMap | Comma-separated domains allowed for post-auth redirects |

### Cookie Override Annotations

| Annotation | Default | Description |
|------------|---------|-------------|
| `spacemule.net/oauth2-proxy.cookie-name` | ConfigMap | Cookie name (useful to prevent collisions) |
| `spacemule.net/oauth2-proxy.cookie-domains` | ConfigMap | Comma-separated cookie domains |
| `spacemule.net/oauth2-proxy.cookie-secure` | ConfigMap | Require HTTPS for cookies (`"true"` or `"false"`) |

### Routing Override Annotations

| Annotation | Default | Description |
|------------|---------|-------------|
| `spacemule.net/oauth2-proxy.redirect-url` | ConfigMap | OAuth callback URL (usually per-service) |
| `spacemule.net/oauth2-proxy.extra-jwt-issuers` | ConfigMap | Comma-separated `issuer=audience` pairs |

### Header Override Annotations

| Annotation | Default | Description |
|------------|---------|-------------|
| `spacemule.net/oauth2-proxy.pass-access-token` | ConfigMap | Pass OAuth access token via `X-Forwarded-Access-Token` |
| `spacemule.net/oauth2-proxy.set-xauthrequest` | ConfigMap | Set `X-Auth-Request-User` and `X-Auth-Request-Email` headers |
| `spacemule.net/oauth2-proxy.pass-authorization-header` | ConfigMap | Pass OIDC ID token via `Authorization: Bearer` header |

### Behavior Override Annotations

| Annotation | Default | Description |
|------------|---------|-------------|
| `spacemule.net/oauth2-proxy.skip-provider-button` | ConfigMap | Skip "Sign in with X" button, redirect directly |

### Provider Override Annotations

| Annotation | Default | Description |
|------------|---------|-------------|
| `spacemule.net/oauth2-proxy.provider` | ConfigMap | OAuth2 provider type (`"oidc"`, `"google"`, `"github"`, etc.) |
| `spacemule.net/oauth2-proxy.oidc-issuer-url` | ConfigMap | OIDC issuer URL |
| `spacemule.net/oauth2-proxy.oidc-groups-claim` | ConfigMap | OIDC claim containing group membership |

### Container Override Annotations

| Annotation | Default | Description |
|------------|---------|-------------|
| `spacemule.net/oauth2-proxy.proxy-image` | ConfigMap | oauth2-proxy container image |

## ConfigMap Keys

| Key | Required | Default | Description |
|-----|----------|---------|-------------|
| `provider` | Yes | - | OAuth2 provider type (`"oidc"`, `"google"`, `"github"`, etc.) |
| `oidc-issuer-url` | Yes* | - | OIDC issuer URL (*required when `provider=oidc`) |
| `oidc-groups-claim` | No | `"groups"` | Claim containing group membership |
| `scope` | No | `"openid email profile"` | OAuth scopes to request |
| `client-id` | Yes | - | OAuth2 client ID |
| `client-secret-ref` | No** | - | Secret reference for client secret (`"secret-name"` or `"secret-name:key"`) |
| `pkce-enabled` | No | `"false"` | Enable PKCE flow (**required if `client-secret-ref` not set) |
| `cookie-secret-ref` | Yes | - | Secret reference for cookie encryption secret |
| `cookie-domains` | No | - | Comma-separated cookie domains |
| `cookie-secure` | No | `"true"` | Require HTTPS for cookies |
| `cookie-name` | No | `"_oauth2_proxy"` | Cookie name |
| `email-domains` | No | - | Comma-separated allowed email domains |
| `allowed-groups` | No | - | Comma-separated allowed groups |
| `whitelist-domains` | No | - | Comma-separated domains allowed for redirects |
| `redirect-url` | No | - | OAuth callback URL |
| `extra-jwt-issuers` | No | - | Comma-separated `issuer=audience` pairs |
| `pass-access-token` | No | `"false"` | Pass OAuth access token to upstream |
| `set-xauthrequest` | No | `"false"` | Set X-Auth-Request-* headers |
| `pass-authorization-header` | No | `"false"` | Pass ID token as Authorization header |
| `skip-provider-button` | No | `"false"` | Skip provider selection button |
| `proxy-image` | No | `"quay.io/oauth2-proxy/oauth2-proxy:v7.14.2"` | oauth2-proxy container image |
| `extra-args` | No | - | Newline-separated extra oauth2-proxy arguments |

## Blocking Direct Access with iptables

When using numbered port mode (service mode), the application container's ports remain accessible directly via the pod IP, potentially bypassing oauth2-proxy authentication. The `block-direct-access` annotation solves this by injecting an init container that configures iptables rules to block direct connections.

### How It Works

1. An init container runs with `NET_ADMIN` capability
2. It creates iptables rules to:
   - Accept traffic from `127.0.0.1` (localhost) to the protected port
   - Drop all other traffic to the protected port
3. Health checks are automatically rewritten to route through oauth2-proxy
4. Only traffic through oauth2-proxy (on port 4180) can reach the protected port

### Example

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    spacemule.net/oauth2-proxy.enabled: "true"
    spacemule.net/oauth2-proxy.protected-port: "8080"
    spacemule.net/oauth2-proxy.block-direct-access: "true"
    spacemule.net/oauth2-proxy.ignore-paths: "/health,/metrics"
spec:
  containers:
  - name: app
    image: myapp:latest
    ports:
    - containerPort: 8080
    livenessProbe:
      httpGet:
        port: 8080  # Automatically rewritten to port 4180
        path: /health
```

### Requirements

- Cluster must allow pods with `NET_ADMIN` capability
- Pod Security Policies/Standards must permit this (if enforced)
- Health check paths should be added to `ignore-paths` to allow Kubelet access

### Health Check Path Conflicts

If your application uses `/ping` or `/ready` paths (oauth2-proxy's defaults), you can customize oauth2-proxy's health check paths:

```yaml
metadata:
  annotations:
    spacemule.net/oauth2-proxy.block-direct-access: "true"
    spacemule.net/oauth2-proxy.ping-path: "/oauth2/ping"
    spacemule.net/oauth2-proxy.ready-path: "/oauth2/ready"
```

## Service Annotations

For Service mutation webhook (used with numbered port mode):

| Annotation | Required | Default | Description |
|------------|----------|---------|-------------|
| `spacemule.net/oauth2-proxy.rewrite-ports` | Yes | - | Comma-separated port names or numbers to route through oauth2-proxy |
| `spacemule.net/oauth2-proxy.proxy-port` | No | `"4180"` | Port where oauth2-proxy listens |


TOCHECK:

Add paths for rewritten healthchecks