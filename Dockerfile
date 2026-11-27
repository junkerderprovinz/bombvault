# syntax=docker/dockerfile:1@sha256:572cebbc2b15e6f84c077742952674889cb63cd5e9b80abdffcbe587b6f19183

# The web, build and tools stages run on the runner's own platform, so only the
# runtime stage of the arm64 image goes through QEMU.
ARG BUILDPLATFORM

FROM --platform=$BUILDPLATFORM node:24-slim@sha256:873f586f3794f52281bd039bf02c048a0be08fde619338f9941bad1eb6b34530 AS web
WORKDIR /src
COPY web/ ./web/
RUN npm --prefix web ci --no-audit --no-fund
# The add-node window shows this compose file as it is written.
COPY deploy/docker-compose.node.yml ./deploy/
RUN npm --prefix web run build

FROM --platform=$BUILDPLATFORM golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea AS build
WORKDIR /src

# Modules first, so the download stays cached across source changes.
COPY go.mod go.sum ./
RUN go mod download

# web/embed.go embeds web/dist, so the built SPA goes there.
COPY cmd ./cmd
COPY internal ./internal
COPY web/*.go ./web/
COPY --from=web /src/web/dist ./web/dist

ARG TARGETOS
ARG TARGETARCH
# Shown in the startup banner; CI passes the release tag.
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags "-s -w -X github.com/junkerderprovinz/bombvault/internal/api.Version=${VERSION}" -o /out/bombvault ./cmd/bombvault

FROM --platform=$BUILDPLATFORM debian:stable-slim@sha256:a64d9dbf633e2a2503afac76be521dc25d9b5765150b30d31a3914d4ae969cab AS tools
# Debian's restic is older than 0.17, which --insecure-no-password needs, and
# its rclone 1.60 breaks some backends (Jottacloud fails restic init), so both
# come from upstream. fetch-tool.sh checks them against upstream's signed
# SHA256SUMS, which is why a version bump is all an update takes.
# renovate: datasource=github-releases depName=restic/restic
ARG RESTIC_VERSION=0.20.0
# renovate: datasource=github-releases depName=rclone/rclone
ARG RCLONE_VERSION=1.76.0
ARG TARGETARCH
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl gpg gpgv bzip2 unzip \
    && rm -rf /var/lib/apt/lists/*
COPY scripts/fetch-tool.sh /fetch/
COPY scripts/keys /fetch/keys
RUN sh /fetch/fetch-tool.sh restic "${RESTIC_VERSION}" "${TARGETARCH}" /out \
    && sh /fetch/fetch-tool.sh rclone "${RCLONE_VERSION}" "${TARGETARCH}" /out

FROM debian:stable-slim@sha256:a64d9dbf633e2a2503afac76be521dc25d9b5765150b30d31a3914d4ae969cab AS runtime

LABEL org.opencontainers.image.title="bombvault" \
      org.opencontainers.image.description="Backup & disaster recovery for Docker containers and KVM/libvirt VMs, powered by restic." \
      org.opencontainers.image.source="https://github.com/junkerderprovinz/bombvault" \
      org.opencontainers.image.licenses="AGPL-3.0-only"

# curl is for the Backup Everything hooks, which run in this container and use
# it for health pings. The upgrade picks up security fixes that reach Debian's
# archive before they reach the pinned base image.
RUN apt-get update \
    && apt-get -y upgrade \
    && apt-get install -y --no-install-recommends ca-certificates libvirt-clients qemu-utils openssh-client tini curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=tools /out/restic /out/rclone /usr/local/bin/
RUN restic version && rclone version

COPY --from=build /out/bombvault /usr/local/bin/bombvault

ENV DATA_DIR=/config \
    HOST_MOUNT_ROOT=/host/user \
    PORT=3000 \
    HTTPS_PORT=3443

# Hours one backup may hold its domain lock before it is cancelled. Empty means
# 48, 0 means no limit.
ENV BACKUP_MAX_HOURS=

VOLUME /config
EXPOSE 3000 3443

# The binary checks its own /api/health, so the healthcheck needs no shell. A
# backup never keeps the API from starting, so 40s covers the cold start.
HEALTHCHECK --interval=30s --timeout=5s --start-period=40s --retries=3 \
    CMD ["/usr/local/bin/bombvault", "healthcheck"]

# A killed restic leaves its rclone child to PID 1, and virsh does the same with
# ssh. tini reaps them, and -g passes SIGTERM to the whole process group so
# docker stop shuts everything down.
ENTRYPOINT ["/usr/bin/tini", "-g", "--", "/usr/local/bin/bombvault"]
