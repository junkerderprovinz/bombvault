#!/usr/bin/env bash
# =============================================================================
# docker-entrypoint.sh — prints the init-log banner, then starts the server.
# =============================================================================
set -e

/usr/local/bin/print-banner.sh \
    "bombvault" \
    "Backup & disaster recovery for Docker containers and KVM/libvirt VMs"

# exec so the Node process becomes PID 1 and receives container signals.
exec node_modules/.bin/tsx custom-server.ts
