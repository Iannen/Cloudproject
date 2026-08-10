#!/usr/bin/env bash
set -euo pipefail

# Define target working directory on the VM
DIR="${HOME:-/root}/bootstrap"
mkdir -p "$DIR"

# Define defaults
MODE="attended" 
TS_KEY=""

# Parse flags safely
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
            echo "Unknown argument: $1"
            echo "Usage: bootstrap.sh [--template --key <key> | --attended | --unattended]"
            exit 1
            ;;
    esac
done

# Validation check for template mode
if [[ "$MODE" == "template" && -z "$TS_KEY" ]]; then
    echo "Error: --template mode requires a validation key via --key <ts_key>"
    exit 1
fi

# ------------------------------------------------------------------------------
# Fetch and extract the asset tarball directly into the working directory
# ------------------------------------------------------------------------------
echo "Fetching core infrastructure assets..."
ASSET_SERVER="http://192.168.50.246:8000"

curl -sSL "$ASSET_SERVER/bootstrap.tar.gz" -o "$DIR/bootstrap.tar.gz"
tar -xzf "$DIR/bootstrap.tar.gz" -C "$DIR"
# ------------------------------------------------------------------------------

run_script() {
    local script_name="$1"
    shift
    
    echo
    echo "=================================================="
    echo "Executing: $script_name ($MODE mode)"
    echo "=================================================="
    
    # Run the extracted scripts from the explicit local directory
    bash "$DIR/$script_name" "$@"
}

# Execute script subsets based on selected mode
case "$MODE" in
    template)
        # Pass the validation key explicitly as an argument
        run_script "create-env.sh" "--template" "--key" "$TS_KEY"
        run_script "install-software.sh"
        ;;

    attended)
        run_script "create-env.sh" "--attended"
        run_script "install-software.sh"
        run_script "join-tailscale.sh" "--attended"
        run_script "join-etcd-cluster.sh"
        ;;

    unattended)
        run_script "create-env.sh" "--unattended"
        run_script "join-tailscale.sh" "--unattended"
        run_script "join-etcd-cluster.sh"
        ;;
esac

echo
echo "Bootstrap complete ($MODE mode)."
