# oauth2-proxy-injector

A Kubernetes mutating admission webhook that injects oauth2-proxy sidecars into pods for authentication.

I used Claude to teach me Go. Claude scaffolded this project, but as of 26 March 2026, some contributions (grunt work, mainly) are offloaded to Claude. All was reviewed, but I am a beginner. Don't use this for security critical purposes if you haven't reviewed it.

See CLAUDE.md and the deploy directory for configuration details.

## Overview

The mutating webhook adds a sidecar running oauth2-proxy targeting the named port (the service must also target the port by name) in properly annotated pods. It adds easy authentication to services without having to edit existing helm-charts or manually add sidecars to manifests.

Configuration is done via:
- **ConfigMap**: Non-sensitive values shared across services in a namespace
- **Annotations**: Service-specific configurations and overrides

## Value Sources

Most configuration values support three source types:

| Source | Annotation Value | Description |
|--------|-----------------|-------------|
| **Literal** | `"my-value"` | Value passed directly as `--flag=my-value` |
| **Environment** | `"fromEnv"` | Flag skipped; oauth2-proxy reads `OAUTH2_PROXY_*` env var at runtime |
| **File** | `"file"` | Uses `--*-file` flag pointing to CSI-mounted file (secrets only) |
| **File (custom path)** | `"file:/path/to/secret"` | Uses `--*-file` flag pointing to specified path (secrets only) |

### Example: Using `fromEnv` with `env-secret`

When you set an annotation to `"fromEnv"`, the webhook skips generating that flag and instead creates an environment variable that reads from a Secret. You must also set `env-secret` to specify which Secret contains the values.

```yaml
metadata:
  annotations:
    spacemule.net/oauth2-proxy.enabled: "true"
    spacemule.net/oauth2-proxy.env-secret: "oauth2-proxy-config"
    spacemule.net/oauth2-proxy.provider: "fromEnv"
    spacemule.net/oauth2-proxy.client-id: "fromEnv"
    spacemule.net/oauth2-proxy.oidc-issuer-url: "fromEnv"
```

The Secret should have keys matching the annotation names:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: oauth2-proxy-config
type: Opaque
stringData:
  provider: "oidc"
  client-id: "my-client-id"
  oidc-issuer-url: "https://auth.example.com/realms/myrealm"
```

The webhook generates env vars like:
```yaml
env:
- name: OAUTH2_PROXY_CLIENT_ID
  valueFrom:
    secretKeyRef:
      name: oauth2-proxy-config
      key: client-id
```

This is useful when:
- Secrets are managed via External Secrets Operator
- Configuration comes from a secrets management system
- You want to decouple config from annotations

### Example: Using CSI Secrets Driver

For secrets via CSI (e.g., Vault CSI Provider, Azure Key Vault), use `"file"` for secrets and specify the `SecretProviderClass`:

```yaml
metadata:
  annotations:
    spacemule.net/oauth2-proxy.enabled: "true"
    spacemule.net/oauth2-proxy.secret-provider-class: "vault-oauth2-secrets"
    spacemule.net/oauth2-proxy.client-secret: "file"
    spacemule.net/oauth2-proxy.cookie-secret: "file"
    # Other config can be literal or fromEnv
    spacemule.net/oauth2-proxy.client-id: "my-client"
    spacemule.net/oauth2-proxy.provider: "oidc"
```

The CSI volume is mounted at `/etc/oauth2-proxy/conf.d/` and secrets are read via `--client-secret-file` and `--cookie-secret-file` flags.

### Example: Using Vault Agent Injector

Vault Agent Injector writes files (not environment variables), so we need a different approach. The webhook supports two mechanisms:

1. **`env-file`**: Source a shell script containing `export` statements before starting oauth2-proxy
2. **`file:/path`**: Specify custom file paths for secrets with native `--*-file` support

**Important**: When using `env-file`, the oauth2-proxy image **must include a shell** (e.g., the Alpine-based images like `quay.io/oauth2-proxy/oauth2-proxy:v7.6.0-alpine`). The default distroless images will not work.

```yaml
metadata:
  annotations:
    # Vault Agent Injector annotations (handled by Vault, not this webhook)
    vault.hashicorp.com/agent-inject: "true"
    vault.hashicorp.com/agent-inject-secret-env: "secret/data/oauth2"
    vault.hashicorp.com/agent-inject-template-env: |
      {{ with secret "secret/data/oauth2" }}
      export OAUTH2_PROXY_CLIENT_ID="{{ .Data.data.client_id }}"
      export OAUTH2_PROXY_OIDC_ISSUER_URL="{{ .Data.data.issuer_url }}"
      {{ end }}
    vault.hashicorp.com/agent-inject-secret-client-secret: "secret/data/oauth2"
    vault.hashicorp.com/agent-inject-template-client-secret: |
      {{ with secret "secret/data/oauth2" }}{{ .Data.data.client_secret }}{{ end }}
    vault.hashicorp.com/agent-inject-secret-cookie-secret: "secret/data/oauth2"
    vault.hashicorp.com/agent-inject-template-cookie-secret: |
      {{ with secret "secret/data/oauth2" }}{{ .Data.data.cookie_secret }}{{ end }}

    # This webhook's annotations
    spacemule.net/oauth2-proxy.enabled: "true"
    spacemule.net/oauth2-proxy.proxy-image: "quay.io/oauth2-proxy/oauth2-proxy:v7.6.0-alpine"

    # Source env file before starting (for non-secret config)
    spacemule.net/oauth2-proxy.env-file: "/vault/secrets/env"
    spacemule.net/oauth2-proxy.client-id: "fromEnv"
    spacemule.net/oauth2-proxy.oidc-issuer-url: "fromEnv"

    # Custom file paths for secrets (using file:/path syntax)
    spacemule.net/oauth2-proxy.client-secret-ref: "file:/vault/secrets/client-secret"
    spacemule.net/oauth2-proxy.cookie-secret-ref: "file:/vault/secrets/cookie-secret"

    # Literal values still work as before
    spacemule.net/oauth2-proxy.provider: "oidc"
    spacemule.net/oauth2-proxy.protected-port: "http"
```

When `env-file` is set, the container command becomes:
```yaml
command: ["/bin/sh", "-c"]
args: ["source /vault/secrets/env && exec /bin/oauth2-proxy --provider=oidc ..."]
```

This sources the environment variables from the Vault-injected file, then starts oauth2-proxy with `exec` to ensure proper signal handling.

### Fields That Don't Support `fromEnv`

Some fields are pod-specific and are used by the webhook at injection time, not by oauth2-proxy at runtime:

- `protected-port` - Which port to proxy
- `upstream-tls` - TLS mode for upstream connection
- `ignore-paths` - Paths to skip authentication
- `api-paths` - Paths requiring JWT only
- `block-direct-access` - Enable iptables protection
- `ping-path` / `ready-path` - Health check endpoints
- `proxy-image` - Container image to use
- `pkce-enabled` - Boolean abstraction (use `code-challenge-method` for `fromEnv`)

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

### Secret Provider Class Annotation

| Annotation | Required | Default | Description |
|------------|----------|---------|-------------|
| `spacemule.net/oauth2-proxy.secret-provider-class` | No | - | Name of SecretProviderClass for CSI secrets driver |

When set, a CSI volume is mounted at `/etc/oauth2-proxy/conf.d/`. Use `"file"` as the value for secret annotations to read from this mount.

### Environment Secret Annotation

| Annotation | Required | Default | Description |
|------------|----------|---------|-------------|
| `spacemule.net/oauth2-proxy.env-secret` | No | - | Name of Secret containing values for `"fromEnv"` fields |
| `spacemule.net/oauth2-proxy.extra-env` | No | - | Additional env vars to inject: `"secretKey:ENV_VAR_NAME,..."` |
| `spacemule.net/oauth2-proxy.env-file` | No | - | Path to env file to source before starting oauth2-proxy (for Vault Agent Injector) |

When `env-secret` is set, fields with `"fromEnv"` source will generate env var entries that read from this Secret. The Secret keys should match annotation names (e.g., `client-id`, `provider`, `oidc-issuer-url`).

When `env-file` is set, the container uses a shell wrapper to source the file before starting oauth2-proxy. This is useful for **Vault Agent Injector** which writes files containing `export` statements. **Important**: The oauth2-proxy image must include a shell (use Alpine-based images like `v7.6.0-alpine`).

The `extra-env` annotation allows injecting arbitrary environment variables from the same Secret. These env vars are available to oauth2-proxy at runtime and can be referenced in literal annotation values using `${VAR_NAME}` syntax.

**Example: Zitadel project ID for group-based access control**

```yaml
metadata:
  annotations:
    spacemule.net/oauth2-proxy.env-secret: "oauth2-config"
    spacemule.net/oauth2-proxy.extra-env: "project-id:PROJECT_ID"
    # Reference the env var in a literal annotation value
    spacemule.net/oauth2-proxy.allowed-groups: "${PROJECT_ID}:admin,${PROJECT_ID}:family"
    # Other config via fromEnv
    spacemule.net/oauth2-proxy.client-id: "fromEnv"
    spacemule.net/oauth2-proxy.oidc-issuer-url: "fromEnv"
```

Secret:
```yaml
stringData:
  client-id: "123456789@my-project"
  oidc-issuer-url: "https://my-instance.zitadel.cloud"
  project-id: "123456789"  # Used by extra-env
```

This injects the `PROJECT_ID` env var into the oauth2-proxy container. The `allowed-groups` annotation generates `--allowed-group=${PROJECT_ID}:admin --allowed-group=${PROJECT_ID}:family`, and oauth2-proxy expands the env var at runtime to `--allowed-group=123456789:admin --allowed-group=123456789:family`.

### Identity Override Annotations

| Annotation | Default | Supports | Description |
|------------|---------|----------|-------------|
| `spacemule.net/oauth2-proxy.client-id` | ConfigMap | `fromEnv` | OAuth2 client ID |
| `spacemule.net/oauth2-proxy.client-secret-ref` | ConfigMap | `file`, `file:/path`, `fromEnv` | Secret reference (`"secret-name"` or `"secret-name:key"`), `"file"` (CSI path), `"file:/custom/path"`, or `"fromEnv"` |
| `spacemule.net/oauth2-proxy.cookie-secret-ref` | ConfigMap | `file`, `file:/path`, `fromEnv` | Secret reference, `"file"` (CSI path), `"file:/custom/path"`, or `"fromEnv"` |
| `spacemule.net/oauth2-proxy.scope` | ConfigMap | `fromEnv` | OAuth scopes to request |
| `spacemule.net/oauth2-proxy.pkce-enabled` | ConfigMap | - | Enable PKCE flow (`"true"` or `"false"`). Sets `--code-challenge-method=S256` |
| `spacemule.net/oauth2-proxy.code-challenge-method` | ConfigMap | `fromEnv` | PKCE code challenge method (`"S256"`, `"plain"`, or `"fromEnv"`) |
| `spacemule.net/oauth2-proxy.validate-url` | ConfigMap | `fromEnv` | Validation URL for opaque tokens |

### Authorization Override Annotations

| Annotation | Default | Supports | Description |
|------------|---------|----------|-------------|
| `spacemule.net/oauth2-proxy.email-domains` | ConfigMap | `fromEnv` | Comma-separated allowed email domains. Use `"*"` for all |
| `spacemule.net/oauth2-proxy.allowed-groups` | ConfigMap | `fromEnv` | Comma-separated allowed groups |
| `spacemule.net/oauth2-proxy.whitelist-domains` | ConfigMap | `fromEnv` | Comma-separated domains allowed for post-auth redirects |

### Cookie Override Annotations

| Annotation | Default | Supports | Description |
|------------|---------|----------|-------------|
| `spacemule.net/oauth2-proxy.cookie-name` | ConfigMap | `fromEnv` | Cookie name (useful to prevent collisions) |
| `spacemule.net/oauth2-proxy.cookie-domains` | ConfigMap | `fromEnv` | Comma-separated cookie domains |
| `spacemule.net/oauth2-proxy.cookie-secure` | ConfigMap | `fromEnv` | Require HTTPS for cookies (`"true"`, `"false"`, or `"fromEnv"`) |

### Routing Override Annotations

| Annotation | Default | Supports | Description |
|------------|---------|----------|-------------|
| `spacemule.net/oauth2-proxy.redirect-url` | ConfigMap | `fromEnv` | OAuth callback URL (usually per-service) |
| `spacemule.net/oauth2-proxy.extra-jwt-issuers` | ConfigMap | `fromEnv` | Comma-separated `issuer=audience` pairs |

### Header Override Annotations

| Annotation | Default | Supports | Description |
|------------|---------|----------|-------------|
| `spacemule.net/oauth2-proxy.pass-access-token` | ConfigMap | `fromEnv` | Pass OAuth access token via `X-Forwarded-Access-Token` |
| `spacemule.net/oauth2-proxy.set-xauthrequest` | ConfigMap | `fromEnv` | Set `X-Auth-Request-User` and `X-Auth-Request-Email` headers |
| `spacemule.net/oauth2-proxy.pass-authorization-header` | ConfigMap | `fromEnv` | Pass OIDC ID token via `Authorization: Bearer` header |

### Behavior Override Annotations

| Annotation | Default | Supports | Description |
|------------|---------|----------|-------------|
| `spacemule.net/oauth2-proxy.skip-provider-button` | ConfigMap | `fromEnv` | Skip "Sign in with X" button, redirect directly |
| `spacemule.net/oauth2-proxy.skip-jwt-bearer-tokens` | `"false"` | `fromEnv` | Skip login when valid JWT bearer token is provided |

### Provider Override Annotations

| Annotation | Default | Supports | Description |
|------------|---------|----------|-------------|
| `spacemule.net/oauth2-proxy.provider` | ConfigMap | `fromEnv` | OAuth2 provider type (`"oidc"`, `"google"`, `"github"`, etc.) |
| `spacemule.net/oauth2-proxy.oidc-issuer-url` | ConfigMap | `fromEnv` | OIDC issuer URL |
| `spacemule.net/oauth2-proxy.oidc-groups-claim` | ConfigMap | `fromEnv` | OIDC claim containing group membership |

### Container Override Annotations

| Annotation | Default | Supports | Description |
|------------|---------|----------|-------------|
| `spacemule.net/oauth2-proxy.proxy-image` | ConfigMap | - | oauth2-proxy container image (no `fromEnv` - used at injection time) |

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


## Full Example: CSI Secrets with Vault

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    # Enable injection
    spacemule.net/oauth2-proxy.enabled: "true"

    # Use Vault CSI driver for secrets
    spacemule.net/oauth2-proxy.secret-provider-class: "vault-oauth2"
    spacemule.net/oauth2-proxy.client-secret: "file"
    spacemule.net/oauth2-proxy.cookie-secret: "file"

    # Non-secret config via fromEnv (injected by Vault Agent)
    spacemule.net/oauth2-proxy.provider: "fromEnv"
    spacemule.net/oauth2-proxy.oidc-issuer-url: "fromEnv"
    spacemule.net/oauth2-proxy.client-id: "fromEnv"

    # Pod-specific settings (literal values)
    spacemule.net/oauth2-proxy.protected-port: "http"
    spacemule.net/oauth2-proxy.redirect-url: "https://myapp.example.com/oauth2/callback"
    spacemule.net/oauth2-proxy.ignore-paths: "/health,/metrics"
spec:
  containers:
  - name: app
    image: myapp:latest
    ports:
    - name: http
      containerPort: 8080
```