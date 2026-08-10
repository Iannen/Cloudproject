#!/usr/bin/env bash
set -euo pipefail

DIR="${HOME:-/root}/bootstrap"; mkdir -p "$DIR"
MODE="attended"; TS_KEY=""

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
            echo "Unknown argument: $1"
            echo "Usage: bootstrap.sh [--template --key <key> | --attended | --unattended]"
            exit 1 ;;
    esac
done

if [[ "$MODE" == "template" && -z "$TS_KEY" ]]; then
    echo "Error: --template mode requires a validation key via --key <ts_key>"; exit 1
fi

echo "Fetching core infrastructure assets..."
curl -sSL "http://192.168.50.246:8000/bootstrap.tar.gz" -o "$DIR/bootstrap.tar.gz"
tar -xzf "$DIR/bootstrap.tar.gz" -C "$DIR"

run() {
    echo -e "\nExecuting: $1 ($MODE mode)"
    bash "$DIR/$1" "${@:2}"
}

case "$MODE" in
    template)
        run "create-env.sh" "--template" "--key" "$TS_KEY"
        run "install-software.sh" ;;
    attended)
        run "create-env.sh" "--attended"
        run "install-software.sh"
        run "join-tailscale.sh" "--attended"
        run "join-etcd-cluster.sh" ;;
    unattended)
        run "create-env.sh" "--unattended"
        run "join-tailscale.sh" "--unattended"
        run "join-etcd-cluster.sh" ;;
esac

echo -e "\nBootstrap complete ($MODE mode)."