# oauth2-proxy-injector multi-stage build
FROM docker.io/golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w" \
    -o oauth2-proxy-webhook \
    ./cmd/webhook

FROM docker.io/alpine:latest

USER nobody

COPY --from=builder /app/oauth2-proxy-webhook /oauth2-proxy-webhook

EXPOSE 8443

ENTRYPOINT ["/oauth2-proxy-webhook"]
