#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# IMPORT: Source create-env.sh to gain access to the centralized set_env_var function
# This safely loads the logic without re-triggering prompt interactions
source "$DIR/create-env.sh"

# 1. Gather dynamic parameters out of our local file and live environment
NODE_ID=$(grep "NODE_ID=" "$DIR/.env" | cut -d'=' -f2 || hostname -s)

echo "Waiting for Tailscale interface to acquire an IP..."
for i in {1..10}; do
    TAILSCALE_IP="$(tailscale ip -4 | head -n1 || true)"
    if [[ -n "$TAILSCALE_IP" ]]; then
        break
    fi
    sleep 1
done

if [[ -z "$TAILSCALE_IP" ]]; then
    echo "ERROR: Timed out waiting for valid Tailscale IP address."
    exit 1
fi

echo
echo "=================================================="
echo "Appending live cluster metrics to local workspace"
echo "=================================================="

# Idempotently update the file properties safely using the centralized helper
set_env_var "TAILSCALE_IP" "${TAILSCALE_IP}"
set_env_var "ETCD_INITIAL_CLUSTER" "${NODE_ID}=http://${TAILSCALE_IP}:2380"
set_env_var "ETCD_INITIAL_CLUSTER_STATE" "new"

echo
echo "Final layout of /root/bootstrap/.env configuration:"
cat "$DIR/.env"

echo
echo "=================================================="
echo "Starting stack"
echo "=================================================="

cd "$DIR"
docker compose pull
docker compose up -d
docker compose ps

echo
echo "Singleton cluster started."
echo
echo "Controller will initialize:"
echo " - /cloud/cluster_id"
echo " - /cloud/status=not_initialized"
echo
echo "Waiting for future cloud election."
