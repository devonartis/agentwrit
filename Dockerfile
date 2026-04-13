# Stage 1: Build broker binary
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Embed VCS info into the binary for reproducibility traces (go build -buildvcs=true
# is default in 1.24, but -trimpath keeps the build reproducible across hosts).
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o broker ./cmd/broker

# Stage 2: Broker image
FROM alpine:3.21 AS broker

# OCI image labels — populated by docker/metadata-action in the release workflow.
# Static labels (title/licenses/vendor) are baked in here so they're correct even
# when someone does a plain `docker build .` locally without the metadata action.
# Dynamic labels (revision/version/created/source) are overridden by the release
# workflow's --label flags; the bake-time defaults below are only used for local
# builds and non-release CI runs.
LABEL org.opencontainers.image.title="AgentWrit" \
      org.opencontainers.image.description="Ephemeral agent credentialing broker — short-lived, scope-attenuated tokens for AI agents" \
      org.opencontainers.image.vendor="devonartis" \
      org.opencontainers.image.licenses="PolyForm-Internal-Use-1.0.0" \
      org.opencontainers.image.source="https://github.com/devonartis/agentwrit" \
      org.opencontainers.image.url="https://github.com/devonartis/agentwrit" \
      org.opencontainers.image.documentation="https://github.com/devonartis/agentwrit/blob/main/README.md"

RUN apk --no-cache add ca-certificates sqlite curl
WORKDIR /root/
COPY --from=builder /app/broker .
EXPOSE 8080
ENTRYPOINT ["./broker"]
