#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-}" 
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

NODE_ID=$(grep "NODE_ID=" "$DIR/.env" | cut -d'=' -f2 || true)
TS_KEY=$(grep "TAILSCALE_AUTH_KEY=" "$DIR/.env" | cut -d'=' -f2 || true)

if [[ -z "$NODE_ID" || -z "$TS_KEY" ]]; then
    echo "ERROR: Foundational configuration metrics are missing from local environment workspace."
    exit 1
fi

echo "Attempting to bring Tailscale interface up ($MODE mode)..."
if tailscale up --authkey="$TS_KEY" --accept-dns; then
    echo "Successfully connected to tailnet."
else
    echo "ERROR: Tailscale authentication routing failed."
    exit 1
fi
