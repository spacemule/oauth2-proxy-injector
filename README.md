# oauth2-proxy-injector

More documentation to follow.

I used Claude to teach me go. Claude scaffolded this project, but all the code was human-written.

See Claude.md and the deploy directory for an idea of how to configure.

The mutating webhook adds a sidecar running oauth2-proxy targeting the named port (the service must also target the port by name at this point) in properly annotated pods. It adds easy authentication to services without having to edit existing helm-charts or manually add sidecars to manifests.

Configuration is done in via configmap for non-sensitive values that are stable across services and via annotation for service-specific configurations.