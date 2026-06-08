# syntax=docker/dockerfile:1
# =============================================================================
# bombvault — backup & disaster recovery for Docker containers + KVM/libvirt VMs
#
# GitHub:  https://github.com/junkerderprovinz/bombvault
# Image:   ghcr.io/junkerderprovinz/bombvault
# License: MIT
#
# Single static Go binary that serves the JSON API + an embedded React SPA and
# shells out to restic over the mounted docker.sock (Docker SDK, no docker-cli).
# Multi-arch amd64 + arm64; buildx provides TARGETOS/TARGETARCH for cross-build.
# =============================================================================

# ---- Stage 1: web (build the React SPA → web/dist) --------------------------
FROM node:24-slim AS web
WORKDIR /src
COPY web/ ./web/
RUN npm --prefix web ci --no-audit --no-fund
RUN npm --prefix web run build

# ---- Stage 2: build (cross-compile the static Go binary) --------------------
FROM golang:1.26-bookworm AS build
WORKDIR /src

# Module graph first so `go mod download` is cached across source changes.
COPY go.mod go.sum ./
RUN go mod download

# Go sources + the web package (embed.go lives at web/). The built dist from
# stage 1 lands at web/dist so the `//go:embed all:dist` in web/embed.go resolves.
COPY cmd ./cmd
COPY internal ./internal
COPY web/*.go ./web/
COPY --from=web /src/web/dist ./web/dist

# buildx injects TARGETOS / TARGETARCH for the requested platform.
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags "-s -w" -o /out/bombvault ./cmd/bombvault

# ---- Stage 3: runtime (lean Debian + restic from upstream release) ----------
FROM debian:stable-slim AS runtime

LABEL org.opencontainers.image.title="bombvault" \
      org.opencontainers.image.description="Backup & disaster recovery for Docker containers and KVM/libvirt VMs, powered by restic." \
      org.opencontainers.image.source="https://github.com/junkerderprovinz/bombvault" \
      org.opencontainers.image.licenses="MIT"

# restic ≥0.17 is required for `--insecure-no-password`; Debian's apt restic is
# too old, so pull the official static binary from GitHub for the target arch.
# (amd64 → linux_amd64, arm64 → linux_arm64.)
ARG RESTIC_VERSION=0.17.3
ARG TARGETARCH
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates bzip2 wget; \
    rm -rf /var/lib/apt/lists/*; \
    case "${TARGETARCH}" in \
        amd64) restic_arch="amd64" ;; \
        arm64) restic_arch="arm64" ;; \
        *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    wget -O /tmp/restic.bz2 \
        "https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_${restic_arch}.bz2"; \
    bunzip2 /tmp/restic.bz2; \
    install -m 0755 /tmp/restic /usr/local/bin/restic; \
    rm -f /tmp/restic; \
    apt-get purge -y --auto-remove bzip2 wget; \
    restic version

COPY --from=build /out/bombvault /usr/local/bin/bombvault

ENV DATA_DIR=/config \
    HOST_MOUNT_ROOT=/host/user \
    PORT=3000 \
    HTTPS_PORT=3443

VOLUME /config
EXPOSE 3000 3443

# The binary prints its own ASCII init + READY banner; no entrypoint script.
ENTRYPOINT ["/usr/local/bin/bombvault"]
