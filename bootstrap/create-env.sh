#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$DIR/.env"

set_env_var() {
    local k="$1" v="$2" f="${3:-$ENV_FILE}"
    touch "$f"
    grep -q "^${k}=" "$f" && sed -i "s|^${k}=.*|${k}=${v}|" "$f" || echo "${k}=${v}" >> "$f"
}

get_env_var() {
    local k="$1" f="${2:-$ENV_FILE}"
    [[ -f "$f" ]] && grep "^${k}=" "$f" | cut -d'=' -f2-
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    MODE="" TS_KEY=""

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --template)   MODE="template"; shift ;;
            --attended)   MODE="attended"; shift ;;
            --unattended) MODE="unattended"; shift ;;
            --key)
                if [[ -n "${2:-}" && ! "$2" =~ ^-- ]]; then
                    TS_KEY="$2"; shift 2
                else
                    echo "Error: --key requires a valid non-empty argument."; exit 1
                fi ;;
            *)
                echo "Unknown argument to create-env.sh: $1"
                echo "Usage: create-env.sh [--template [--key <key>] | --attended | --unattended]"
                exit 1 ;;
        esac
    done

    [[ -n "$MODE" ]] || { echo "ERROR: Mode flag required (--template, --attended, --unattended)"; exit 1; }

    CURR_HOST=$(hostname -s)
    [[ "$CURR_HOST" =~ ^kaffcloud- ]] && FINAL_HOST="$CURR_HOST" || FINAL_HOST="kaffcloud-${CURR_HOST}"

    if [[ "$MODE" != "template" && "$CURR_HOST" != "$FINAL_HOST" ]]; then
        echo "Renaming host to $FINAL_HOST..."
        hostnamectl set-hostname "$FINAL_HOST"
        grep -qE "\b${FINAL_HOST}\b" /etc/hosts || echo "127.0.1.1 $FINAL_HOST" >> /etc/hosts
        udevadm settle || true
    fi

    case "$MODE" in
        unattended)
            TS_KEY=$(get_env_var "TAILSCALE_AUTH_KEY")
            [[ -n "$TS_KEY" ]] || { echo "ERROR: Unattended execution lacks Tailscale key."; exit 1; } ;;
        attended)
            while [[ -z "$TS_KEY" ]]; do
                read -rp "Enter Tailscale auth key: " TS_KEY
            done ;;
        template)
            [[ -n "$TS_KEY" ]] || { echo "ERROR: --template requires --key."; exit 1; } ;;
    esac

    echo "Generating environment configuration..."
    touch "$ENV_FILE" && chmod 600 "$ENV_FILE"
    set_env_var "NODE_ID" "$FINAL_HOST"
    set_env_var "TAILSCALE_AUTH_KEY" "$TS_KEY"

    echo "Environment initialized ($MODE mode)."
fi