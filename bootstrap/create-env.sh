#!/usr/bin/env bash
set -euo pipefail

# Determine the absolute directory of this script context
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$DIR/.env"

# ==============================================================================
# EXPOSED FUNCTIONS: Available to any script that sources this file
# ==============================================================================
set_env_var() {
    local key="$1"
    local value="$2"
    local target_file="${3:-$ENV_FILE}"

    # Ensure the target file exists so grep doesn't break
    touch "$target_file"

    if grep -q "^${key}=" "$target_file"; then
        # Update existing key safely using | as delimiter to avoid escaping slash issues
        sed -i "s|^${key}=.*|${key}=${value}|" "$target_file"
    else
        # Append new key
        echo "${key}=${value}" >> "$target_file"
    fi
}

get_env_var() {
    local key="$1"
    local target_file="${2:-$ENV_FILE}"

    if [[ -f "$target_file" ]]; then
        grep "^${key}=" "$target_file" | cut -d'=' -f2-
    fi
}

# ==============================================================================
# INIT EXECUTION BLOCK: Only executes if run directly, NOT when sourced
# ==============================================================================
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    
    MODE=""
    TS_KEY=""

    # Parse explicit mode and keys passed from bootstrap.sh
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --template)   MODE="template"; shift ;;
            --attended)   MODE="attended"; shift ;;
            --unattended) MODE="unattended"; shift ;;
            --key)
                if [[ -n "${2:-}" && ! "$2" =~ ^-- ]]; then
                    TS_KEY="$2"
                    shift 2
                else
                    echo "Error: --key requires a valid non-empty argument."
                    exit 1
                fi
                ;;
            *)
                echo "Unknown argument to create-env.sh: $1"
                echo "Usage: create-env.sh [--template [--key <key>] | --attended | --unattended]"
                exit 1
                ;;
        esac
    done

    if [[ -z "$MODE" ]]; then
        echo "ERROR: create-env.sh requires an explicit mode flag: --template, --attended, or --unattended"
        exit 1
    fi

    # 1. Identity Pre-calculation
    CURRENT_HOSTNAME=$(hostname -s)
    if [[ ! "$CURRENT_HOSTNAME" =~ ^kaffcloud- ]]; then
        FINAL_HOSTNAME="kaffcloud-${CURRENT_HOSTNAME}"
    else
        FINAL_HOSTNAME="$CURRENT_HOSTNAME"
    fi

    # 2. Update live OS Hostname (Safely bypassed in template mode)
    if [[ "$MODE" != "template" ]]; then
        if [[ "$CURRENT_HOSTNAME" != "$FINAL_HOSTNAME" ]]; then
            echo "Renaming host to match project standard configuration identifier..."
            hostnamectl set-hostname "$FINAL_HOSTNAME"
            
            # Map hostname locally to stop sudo/systemd warnings
            if ! grep -q "$FINAL_HOSTNAME" /etc/hosts; then
                echo "127.0.1.1 $FINAL_HOSTNAME" >> /etc/hosts
            fi
            udevadm settle || true
        fi
    fi

    # 3. Extract Context/Keys based on the explicit mode
    if [[ "$MODE" == "unattended" ]]; then

        TS_KEY=$(get_env_var "TAILSCALE_AUTH_KEY")

        if [[ -z "$TS_KEY" ]]; then
            echo "ERROR: Unattended execution configuration lacks a baked Tailscale token."
            exit 1
        fi
    elif [[ "$MODE" == "attended" ]]; then
        while true; do
            echo
            read -rp "Enter Tailscale auth key (Ctrl+C to cancel): " TS_KEY
            echo
            if [[ -n "$TS_KEY" ]]; then
                break
            fi
            echo "Key cannot be empty. Please try again."
        done
    elif [[ "$MODE" == "template" ]]; then
        # Ensure template mode actually received the key passed from bootstrap.sh
        if [[ -z "$TS_KEY" ]]; then
            echo "ERROR: --template mode requires a validation key passed via --key"
            exit 1
        fi
    fi

    # 4. Populate baseline configuration using our exposed function
    echo "Generating baseline project environment configuration file..."
    touch "$ENV_FILE"
    chmod 600 "$ENV_FILE"

    set_env_var "NODE_ID" "${FINAL_HOSTNAME}"
    set_env_var "TAILSCALE_AUTH_KEY" "${TS_KEY}"

    echo "Foundational /root/bootstrap/.env successfully initialized ($MODE mode)."
fi
