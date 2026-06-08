# syntax=docker/dockerfile:1
# =============================================================================
# bombvault — backup & disaster recovery for Docker containers + KVM/libvirt VMs
#
# GitHub:  https://github.com/junkerderprovinz/bombvault
# Image:   ghcr.io/junkerderprovinz/bombvault
# License: MIT
#
# Debian-slim base so better-sqlite3/argon2 native addons build, and so the
# bundled host CLIs (restic, docker-cli, virsh, qemu-img, rclone) install cleanly.
# Multi-arch amd64 + arm64.
# =============================================================================
ARG NODE_VERSION=22

# ---- Stage 1: deps (toolchain + native addons) ------------------------------
FROM node:${NODE_VERSION}-slim AS deps
WORKDIR /app
RUN apt-get update \
    && apt-get install -y --no-install-recommends python3 make g++ \
    && rm -rf /var/lib/apt/lists/*
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

# ---- Stage 2: build ---------------------------------------------------------
FROM node:${NODE_VERSION}-slim AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

# ---- Stage 3: runtime -------------------------------------------------------
FROM node:${NODE_VERSION}-slim AS runtime
WORKDIR /app

ENV NODE_ENV=production \
    DATA_DIR=/data \
    CONFIG_DIR=/config \
    PORT=3000 \
    HTTPS_PORT=3443 \
    HOSTNAME=0.0.0.0

LABEL org.opencontainers.image.title="bombvault" \
      org.opencontainers.image.description="Backup & disaster recovery for Docker containers and KVM/libvirt VMs, powered by restic." \
      org.opencontainers.image.source="https://github.com/junkerderprovinz/bombvault" \
      org.opencontainers.image.licenses="MIT" \
      maintainer="junkerderprovinz"

# Bundled host CLIs the app shells out to + openssl for the self-signed cert.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        restic docker.io libvirt-clients qemu-utils rclone openssl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /app/node_modules ./node_modules
COPY --from=build /app/.next ./.next
COPY package.json package-lock.json next.config.mjs tsconfig.json next-env.d.ts custom-server.ts ./
COPY lib ./lib
COPY server ./server
COPY app ./app
COPY scripts ./scripts

# Init-log banner (brand art shared; container name passed at runtime).
COPY .github/assets/banner-raw.txt /usr/local/share/banner-raw.txt
COPY print-banner.sh /usr/local/bin/print-banner.sh
COPY docker-entrypoint.sh ./docker-entrypoint.sh
RUN tr -d '\r' < /usr/local/share/banner-raw.txt > /usr/local/share/banner.txt \
    && tr -d '\r' < /usr/local/bin/print-banner.sh > /usr/local/bin/print-banner.sh.tmp \
    && mv /usr/local/bin/print-banner.sh.tmp /usr/local/bin/print-banner.sh \
    && tr -d '\r' < ./docker-entrypoint.sh > ./docker-entrypoint.sh.tmp \
    && mv ./docker-entrypoint.sh.tmp ./docker-entrypoint.sh \
    && chmod +x /usr/local/bin/print-banner.sh ./docker-entrypoint.sh

VOLUME /data /config
EXPOSE 3000 3443

ENTRYPOINT ["./docker-entrypoint.sh"]
