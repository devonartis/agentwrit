# Stage 1: Build broker binary
# Pinned by digest (Dependabot docker ecosystem rotates weekly). Tag: golang:1.26.10-alpine
# NOTE: official golang images set GOTOOLCHAIN=local, so the go.mod `toolchain`
# directive is ignored in this build — the image's own Go version (1.26.10 at this
# digest) is what the binary's stdlib comes from. Keep this tag in lockstep with
# go.mod's toolchain directive or container scans will flag stdlib CVEs that the
# toolchain directive appears (wrongly) to have already fixed.
FROM golang:1.26.10-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Embed VCS info into the binary for reproducibility traces (go build -buildvcs=true
# is default in 1.24, but -trimpath keeps the build reproducible across hosts).
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o broker ./cmd/broker

# Stage 2: Broker image
# Pinned by digest (Dependabot docker ecosystem rotates weekly). Tag: alpine:3.21 (3.21.8)
FROM alpine:3.21@sha256:ce64758a109eb420d874a118f87920e625e12d3634e03b4a5573fd9f6e5d3507 AS broker

# OCI image labels — populated by docker/metadata-action in the release workflow.
# Static labels (title/licenses/vendor) are baked in here so they're correct even
# when someone does a plain `docker build .` locally without the metadata action.
# Dynamic labels (revision/version/created/source) are overridden by the release
# workflow's --label flags; the bake-time defaults below are only used for local
# builds and non-release CI runs.
LABEL org.opencontainers.image.title="AgentWrit" \
      org.opencontainers.image.description="Ephemeral agent credentialing broker — short-lived, scope-attenuated tokens for AI agents" \
      org.opencontainers.image.vendor="devonartis" \
      org.opencontainers.image.licenses="LicenseRef-PolyForm-Internal-Use-1.0.0" \
      org.opencontainers.image.source="https://github.com/devonartis/agentwrit" \
      org.opencontainers.image.url="https://github.com/devonartis/agentwrit" \
      org.opencontainers.image.documentation="https://github.com/devonartis/agentwrit/blob/main/README.md"

# `apk upgrade` pulls security fixes published after the base digest was cut
# (e.g. OpenSSL 3.3.7-r2) — container-scan blocks on any fixable HIGH/CRITICAL.
RUN apk --no-cache upgrade && apk --no-cache add ca-certificates sqlite curl
WORKDIR /root/
COPY --from=builder /app/broker .
EXPOSE 8080
ENTRYPOINT ["./broker"]
