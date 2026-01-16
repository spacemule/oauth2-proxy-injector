# oauth2-proxy-injector Makefile
# TODO: Update IMAGE_REGISTRY to your container registry

# Build configuration
BINARY_NAME := oauth2-proxy-webhook
IMAGE_REGISTRY ?= your-registry
IMAGE_NAME := $(IMAGE_REGISTRY)/oauth2-proxy-webhook
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GOFLAGS := -ldflags "-X main.version=$(VERSION)"

# Kubernetes configuration
NAMESPACE := oauth2-proxy-system

.PHONY: all build test clean container-build container-push deploy undeploy certs fmt vet lint help

# Default target
all: fmt vet test build

##@ Development

# Build the binary for local development
build:
	@echo "Building $(BINARY_NAME)..."
	go build $(GOFLAGS) -o bin/$(BINARY_NAME) ./cmd/webhook

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

# Run tests with coverage report
coverage: test
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Vet code
vet:
	@echo "Vetting code..."
	go vet ./...

# Lint code (requires golangci-lint)
# TODO: Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
lint:
	@echo "Linting code..."
	golangci-lint run ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

##@ Container

# Build container image
container-build:
	@echo "Building container image $(IMAGE_NAME):$(VERSION)..."
	podman build -f Containerfile -t $(IMAGE_NAME):$(VERSION) -t $(IMAGE_NAME):latest .

# Push container image to registry
container-push: container-build
	@echo "Pushing container image..."
	podman push $(IMAGE_NAME):$(VERSION)
	podman push $(IMAGE_NAME):latest

##@ Deployment

# Generate TLS certificates for the webhook
# TODO: For production, use cert-manager instead!
certs:
	@echo "Generating self-signed TLS certificates..."
	@mkdir -p certs
	# Generate CA
	openssl genrsa -out certs/ca.key 2048
	openssl req -new -x509 -days 365 -key certs/ca.key -subj "/CN=oauth2-proxy-webhook-ca" -out certs/ca.crt
	# Generate server certificate
	openssl genrsa -out certs/tls.key 2048
	openssl req -new -key certs/tls.key -subj "/CN=oauth2-proxy-webhook.$(NAMESPACE).svc" \
		-addext "subjectAltName=DNS:oauth2-proxy-webhook,DNS:oauth2-proxy-webhook.$(NAMESPACE),DNS:oauth2-proxy-webhook.$(NAMESPACE).svc" \
		-out certs/server.csr
	openssl x509 -req -in certs/server.csr -CA certs/ca.crt -CAkey certs/ca.key -CAcreateserial -days 365 \
		-extfile <(printf "subjectAltName=DNS:oauth2-proxy-webhook,DNS:oauth2-proxy-webhook.$(NAMESPACE),DNS:oauth2-proxy-webhook.$(NAMESPACE).svc") \
		-out certs/tls.crt
	@echo "Certificates generated in certs/ directory"
	@echo "CA Bundle (base64): $$(cat certs/ca.crt | base64 -w0)"

# Create namespace and deploy webhook
deploy: container-push
	@echo "Deploying webhook to $(NAMESPACE)..."
	# Create namespace
	kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	# Create TLS secret (requires certs to exist)
	kubectl create secret tls oauth2-proxy-webhook-tls \
		--cert=certs/tls.crt \
		--key=certs/tls.key \
		-n $(NAMESPACE) \
		--dry-run=client -o yaml | kubectl apply -f -
	# Apply RBAC
	kubectl apply -f deploy/webhook-rbac.yaml
	# Apply deployment and service
	kubectl apply -f deploy/webhook-deployment.yaml
	kubectl apply -f deploy/webhook-service.yaml
	# Update CA bundle in webhook config and apply
	@CA_BUNDLE=$$(cat certs/ca.crt | base64 -w0) && \
		sed "s/CA_BUNDLE_BASE64/$$CA_BUNDLE/" deploy/mutatingwebhook.yaml | kubectl apply -f -
	@echo "Webhook deployed! Waiting for rollout..."
	kubectl rollout status deployment/oauth2-proxy-webhook -n $(NAMESPACE)

# Remove webhook from cluster
undeploy:
	@echo "Removing webhook..."
	kubectl delete -f deploy/mutatingwebhook.yaml --ignore-not-found
	kubectl delete -f deploy/webhook-deployment.yaml --ignore-not-found
	kubectl delete -f deploy/webhook-service.yaml --ignore-not-found
	kubectl delete -f deploy/webhook-rbac.yaml --ignore-not-found
	kubectl delete secret oauth2-proxy-webhook-tls -n $(NAMESPACE) --ignore-not-found
	@echo "Webhook removed"

# View webhook logs
logs:
	kubectl logs -f -l app=oauth2-proxy-webhook -n $(NAMESPACE)

##@ Local Development

# Run webhook locally (for development)
# TODO: Requires kubeconfig with access to cluster
run-local: build
	@echo "Running webhook locally..."
	./bin/$(BINARY_NAME) \
		--port=8443 \
		--cert-file=certs/tls.crt \
		--key-file=certs/tls.key

##@ Dependencies

# Download Go module dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Update dependencies
deps-update:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

##@ Help

# Show this help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
