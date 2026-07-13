# syntax=docker/dockerfile:1
# =============================================================================
# bombvault — backup & disaster recovery for Docker containers + KVM/libvirt VMs
#
# GitHub:  https://github.com/junkerderprovinz/bombvault
# Image:   ghcr.io/junkerderprovinz/bombvault
# License: MIT
#
# Single static Go binary that serves the JSON API + an embedded React SPA and
# shells out to restic over the mounted docker.sock (Docker SDK, no docker-cli)
# and to virsh for KVM/libvirt VM backup/restore.
# Multi-arch amd64 + arm64; buildx provides TARGETOS/TARGETARCH for cross-build.
# =============================================================================

# buildx injects BUILDPLATFORM (the runner's native platform). Pinning the web
# and build stages to it makes them run NATIVELY and cross-compile, instead of
# being emulated under slow QEMU for the arm64 target.
ARG BUILDPLATFORM

# ---- Stage 1: web (build the React SPA → web/dist) --------------------------
# Arch-independent JS output: build once on the native runner platform.
FROM --platform=$BUILDPLATFORM node:24-slim AS web
WORKDIR /src
COPY web/ ./web/
RUN npm --prefix web ci --no-audit --no-fund
RUN npm --prefix web run build

# ---- Stage 2: build (cross-compile the static Go binary) --------------------
# Runs natively on BUILDPLATFORM and cross-compiles via GOOS/GOARCH (set below).
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS build
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
# VERSION is stamped into the binary (printed in the startup banner + READY box)
# so the running image's version is obvious in the container log. Defaults to
# "dev" for un-stamped local builds; CI passes the release tag.
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags "-s -w -X github.com/junkerderprovinz/bombvault/internal/api.Version=${VERSION}" -o /out/bombvault ./cmd/bombvault

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
# rclone: Debian's apt package is far too old (v1.60.1) and fails on some backends
# — e.g. Jottacloud returns HTTP 500 "AllocationException" on `restic init` (#32) —
# so pull the official current static binary instead, same approach as restic.
# NOTE: rclone reads RCLONE_* env vars as flag overrides, so this build ARG shadows
# rclone's --version flag; the `rclone version` check below runs with it unset.
ARG RCLONE_VERSION=1.74.2
ARG TARGETARCH
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates libvirt-clients qemu-utils openssh-client tini bzip2 wget unzip; \
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
    wget -O /tmp/rclone.zip \
        "https://downloads.rclone.org/v${RCLONE_VERSION}/rclone-v${RCLONE_VERSION}-linux-${restic_arch}.zip"; \
    unzip -j /tmp/rclone.zip "rclone-v${RCLONE_VERSION}-linux-${restic_arch}/rclone" -d /tmp; \
    install -m 0755 /tmp/rclone /usr/local/bin/rclone; \
    rm -f /tmp/rclone.zip /tmp/rclone; \
    apt-get purge -y --auto-remove bzip2 wget unzip; \
    restic version; \
    env -u RCLONE_VERSION rclone version

COPY --from=build /out/bombvault /usr/local/bin/bombvault

ENV DATA_DIR=/config \
    HOST_MOUNT_ROOT=/host/user \
    PORT=3000 \
    HTTPS_PORT=3443

VOLUME /config
EXPOSE 3000 3443

# Docker healthcheck (#60): the engine answers its own /api/health while serving,
# so auto-heal tools (Autoheal etc.) can restart a wedged container. It runs the
# binary itself (`bombvault healthcheck`), so the image needs no shell or curl.
# start-period covers the cold start (store open + first sweep); a backup never
# blocks the API from binding, so 40s is plenty.
HEALTHCHECK --interval=30s --timeout=5s --start-period=40s --retries=3 \
    CMD ["/usr/local/bin/bombvault", "healthcheck"]

# tini is PID 1 (the container init) so orphaned grandchild processes get reaped.
# BombVault shells out to restic, which forks its own `rclone` child for off-site
# repos; when restic is cancelled/killed (a cancelled backup, the off-site check
# timeout, a WAN drop) restic dies and its rclone child is orphaned onto PID 1.
# BombVault-as-PID-1 was not a reaping init, so those piled up as `[rclone]
# <defunct>` (#35). tini reaps them (and virsh→ssh orphans) automatically; `-g`
# forwards SIGTERM/SIGINT to the whole process group for a clean `docker stop`.
# The binary still prints its own ASCII init + READY banner; no entrypoint script.
ENTRYPOINT ["/usr/bin/tini", "-g", "--", "/usr/local/bin/bombvault"]
