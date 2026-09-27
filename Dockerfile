# syntax=docker/dockerfile:1@sha256:ecfaec9ed6d810b56388c508f4121597bfbba70d41a6dfeee4d8cad5f295fc32

# The web and build stages run on the runner's own platform and cross-compile,
# so the arm64 image is not built under QEMU.
ARG BUILDPLATFORM

FROM --platform=$BUILDPLATFORM node:24-slim@sha256:0e0ff40c39bc087845bfb27465a0df4ea419520094bc35842ff83dd8cbe6f9b6 AS web
WORKDIR /src
COPY web/ ./web/
RUN npm --prefix web ci --no-audit --no-fund
RUN npm --prefix web run build

FROM --platform=$BUILDPLATFORM golang:1.27-bookworm@sha256:69a7b9788769bec032d238959b61854e9ae87f57be9029ec04e9885fabf99195 AS build
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

FROM debian:stable-slim@sha256:5bc3287b25407c965a30f38e32603dc253a3869e1b12a21ac09bfc27fd8b13ce AS runtime

LABEL org.opencontainers.image.title="bombvault" \
      org.opencontainers.image.description="Backup & disaster recovery for Docker containers and KVM/libvirt VMs, powered by restic." \
      org.opencontainers.image.source="https://github.com/junkerderprovinz/bombvault" \
      org.opencontainers.image.licenses="AGPL-3.0-only"

# Debian's restic is older than 0.17, which --insecure-no-password needs, so the
# upstream binary is used instead.
ARG RESTIC_VERSION=0.17.3
# Debian's rclone 1.60 breaks some backends (Jottacloud fails restic init), so
# rclone comes from upstream too. rclone reads RCLONE_* variables as flags,
# which is why the version check below runs with these build args unset.
ARG RCLONE_VERSION=1.75.1
# From upstream's SHA256SUMS for the versions above
# (https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/SHA256SUMS
# and https://downloads.rclone.org/v${RCLONE_VERSION}/SHA256SUMS). Update them
# with every version bump, or the build fails.
ARG RESTIC_SHA256_AMD64=5097faeda6aa13167aae6e36efdba636637f8741fed89bbf015678334632d4d3
ARG RESTIC_SHA256_ARM64=db27b803534d301cef30577468cf61cb2e242165b8cd6d8cd6efd7001be2e557
ARG RCLONE_SHA256_AMD64=982b5aa772841168f8e380f139e9e787b2a105403e32b94da8676a0e1c0a13ab
ARG RCLONE_SHA256_ARM64=03f2504174034b6d004152ed7369251c9a9ec1f7e0836eda420f5c7a5ec0dff9
ARG TARGETARCH
# pipefail so no pipe below can hide a failure. bash is essential in Debian,
# so the slim image has it.
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
# curl survives the purge below because the Backup Everything hooks run in this
# container and use it for health pings.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates libvirt-clients qemu-utils openssh-client tini curl bzip2 wget unzip; \
    rm -rf /var/lib/apt/lists/*; \
    case "${TARGETARCH}" in \
        amd64) restic_arch="amd64"; restic_sha256="${RESTIC_SHA256_AMD64}"; rclone_sha256="${RCLONE_SHA256_AMD64}" ;; \
        arm64) restic_arch="arm64"; restic_sha256="${RESTIC_SHA256_ARM64}"; rclone_sha256="${RCLONE_SHA256_ARM64}" ;; \
        *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    wget -q -O /tmp/restic.bz2 \
        "https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_${restic_arch}.bz2"; \
    echo "${restic_sha256}  /tmp/restic.bz2" | sha256sum -c -; \
    bunzip2 /tmp/restic.bz2; \
    install -m 0755 /tmp/restic /usr/local/bin/restic; \
    rm -f /tmp/restic; \
    wget -q -O /tmp/rclone.zip \
        "https://downloads.rclone.org/v${RCLONE_VERSION}/rclone-v${RCLONE_VERSION}-linux-${restic_arch}.zip"; \
    echo "${rclone_sha256}  /tmp/rclone.zip" | sha256sum -c -; \
    unzip -j /tmp/rclone.zip "rclone-v${RCLONE_VERSION}-linux-${restic_arch}/rclone" -d /tmp; \
    install -m 0755 /tmp/rclone /usr/local/bin/rclone; \
    rm -f /tmp/rclone.zip /tmp/rclone; \
    apt-get purge -y --auto-remove bzip2 wget unzip; \
    restic version; \
    env -u RCLONE_VERSION -u RCLONE_SHA256_AMD64 -u RCLONE_SHA256_ARM64 rclone version

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
