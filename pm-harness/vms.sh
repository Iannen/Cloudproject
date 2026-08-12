#!/usr/bin/env bash
set -euo pipefail

CLOUD_NODE_PREFIX="kaffcloud"
PROJECT_ROOT="/root/cloudproject"
BOOTSTRAP_DIR="$PROJECT_ROOT/bootstrap"
CACHE_DIR="/var/lib/vz/template/iso"
STORAGE="local-zfs"
BRIDGE="vmbr0"

TEMPLATE_CORES=2; TEMPLATE_MEMORY=4096; TEMPLATE_DISK=20
VM_CORES=4; VM_MEMORY=3072; VM_DISK=100
DEFAULT_IMAGE_URL="https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img"
DEFAULT_USER="user"; DEFAULT_PASS="pass"

RAW_BOOTSTRAP_URL="https://raw.githubusercontent.com/Iannen/Cloudproject/main/bootstrap/bootstrap.sh"

if [[ -f "/etc/environment" ]]; then
    set -o allexport
    source /etc/environment
    set +o allexport
fi

usage() {
    cat <<EOF
Usage:
  $0 setup
  $0 create-template <template_name> [url] [cores] [memory] [disk]
  $0 create-vm <vm_name> <template_vmid> [cores] [memory] [disk]
  $0 destroy <vmid>
EOF
    exit 1
}

ACTION="${1:-}"; [[ -n "$ACTION" ]] || usage; shift

vm_exists_id() { qm status "$1" >/dev/null 2>&1; }
next_vmid() { pvesh get /cluster/nextid; }

next_template_vmid() {
    local max_id
    max_id=$(pvesh get /cluster/resources --type vm --output-format json | jq -r '.[].vmid' 2>/dev/null | awk '$1>=9000' | sort -n | tail -n1)
    echo "${max_id:-8999}" | awk '{print $1+1}'
}

setup_environment() {
    echo "[+] Running setup check for /etc/environment..."
    local updated=0

    if [[ -z "${TAILSCALE_AUTH_KEY:-}" ]]; then
        read -rp "    Enter Tailscale Auth Key: " input_auth
        if [[ -n "$input_auth" ]]; then
            echo "TAILSCALE_AUTH_KEY=\"$input_auth\"" >> /etc/environment
            export TAILSCALE_AUTH_KEY="$input_auth"
            updated=1
        else
            echo "ERROR: Auth key is required."; exit 1
        fi
    else
        echo "  - TAILSCALE_AUTH_KEY is already set."
    fi

    if [[ -z "${TAILSCALE_API_KEY:-}" ]]; then
        read -rp "    Enter Tailscale API Key: " input_api
        if [[ -n "$input_api" ]]; then
            echo "TAILSCALE_API_KEY=\"$input_api\"" >> /etc/environment
            export TAILSCALE_API_KEY="$input_api"
            updated=1
        else
            echo "ERROR: API key is required."; exit 1
        fi
    else
        echo "  - TAILSCALE_API_KEY is already set."
    fi

    if [[ -z "${TAILSCALE_TAILNET:-}" ]]; then
        local auto_tailnet
        auto_tailnet=$(tailscale status --json 2>/dev/null | jq -r '.MagicDNSSuffix' 2>/dev/null || true)
        
        local prompt_default="${auto_tailnet:-tailca68f1.ts.net}"
        read -rp "    Enter Tailnet Name [Default: ${prompt_default}]: " input_net
        local final_net="${input_net:-$prompt_default}"

        echo "TAILSCALE_TAILNET=\"$final_net\"" >> /etc/environment
        export TAILSCALE_TAILNET="$final_net"
        updated=1
    else
        echo "  - TAILSCALE_TAILNET is already set."
    fi

    if [[ "$updated" -eq 1 ]]; then
        echo "[+] Saved new credentials to /etc/environment."
    else
        echo "[+] Environment is already fully configured!"
    fi
}

clean_tailscale_node() {
    local target_hostname="$1"
    
    if [[ -z "${TAILSCALE_API_KEY:-}" || -z "${TAILSCALE_TAILNET:-}" ]]; then
        echo "[!] Skipping Tailscale cleanup: TAILSCALE_API_KEY or TAILSCALE_TAILNET missing from environment."
        return 0
    fi

    echo "[-] Searching Tailscale API for device matching: ${target_hostname}..."
    
    local device_id
    device_id=$(curl -s -u "${TAILSCALE_API_KEY}:" \
        "https://api.tailscale.com/api/v2/tailnet/${TAILSCALE_TAILNET}/devices" \
        | jq -r ".devices[]? | select(.hostname == \"${target_hostname}\") | .id")

    if [[ -n "$device_id" && "$device_id" != "null" ]]; then
        echo "[-] Issuing instant DELETE request for Tailscale device ID $device_id..."
        curl -s -X DELETE -u "${TAILSCALE_API_KEY}:" \
            "https://api.tailscale.com/api/v2/device/${device_id}"
        echo "[+] Tailscale device instantly wiped."
    else
        echo "[!] No matching device found in Tailnet."
    fi
}

create_template() {
    if [[ -z "${TAILSCALE_AUTH_KEY:-}" ]]; then
        echo "ERROR: TAILSCALE_AUTH_KEY is missing. Run '$0 setup' first."
        exit 1
    fi

    local template_name="${1:-}"; [[ -n "$template_name" ]] || usage; shift

    local img_url="${1:-$DEFAULT_IMAGE_URL}"
    local cores="${2:-$TEMPLATE_CORES}" memory="${3:-$TEMPLATE_MEMORY}" disk="${4:-$TEMPLATE_DISK}"

    local vmid; vmid=$(next_template_vmid)
    local full_name="${template_name}-${vmid}" 
    local tmp_img="/tmp/cloud_${vmid}.img"
    local snippet="/var/lib/vz/snippets/user-data-${vmid}.yml"

    echo "[+] Building template $vmid ($full_name)..."
    curl -L -s -o "$tmp_img" "$img_url"

    qm create "$vmid" --name "$full_name" --memory "$memory" --cores "$cores" --cpu host \
        --machine q35 --bios ovmf --net0 virtio,bridge="$BRIDGE" --scsihw virtio-scsi-single \
        --serial0 socket --vga serial0

    qm importdisk "$vmid" "$tmp_img" "$STORAGE"
    qm set "$vmid" --scsi0 "$STORAGE:vm-$vmid-disk-0"

    mkdir -p /var/lib/vz/snippets
cat <<EOF > "$snippet"
#cloud-config
user: $DEFAULT_USER
password: $DEFAULT_PASS
chpasswd: { expire: False }
ssh_pwauth: True
hostname: ${full_name}
fqdn: ${full_name}.local
runcmd:
  - |
    exec > /var/log/cloud-init-bootstrap.log 2>&1
    echo "[+] Waiting for network connection..."
    until ping -c 1 8.8.8.8; do sleep 2; done
    
    echo "[+] Executing Remote Bootstrap Script..."
    curl -sSL "https://raw.githubusercontent.com/Iannen/Cloudproject/refs/heads/main/bootstrap/bootstrap.sh" | bash -s -- --template --key "${TAILSCALE_AUTH_KEY}"
    
    echo "[+] Bootstrap completed successfully. Shutting down..."
    systemctl poweroff
EOF

    qm set "$vmid" --efidisk0 "$STORAGE:1,efitype=4m,pre-enrolled-keys=1" \
        --ide2 "$STORAGE:cloudinit" \
        --ciuser "$DEFAULT_USER" --cipassword "$DEFAULT_PASS" --ipconfig0 ip=dhcp \
        --cicustom "user=local:snippets/user-data-${vmid}.yml"

    qm set "$vmid" --boot "order=scsi0"

    qm resize "$vmid" scsi0 "${disk}G"
    rm -f "$tmp_img"

    qm start "$vmid"
    
    echo "[+] Waiting for VM $vmid to execute Cloud-Init and shut down..."
    while qm status "$vmid" | grep -q running; do sleep 3; done

    qm template "$vmid"
    rm -f "$snippet"
    echo "[+] Template blueprint created successfully: $vmid"
}

create_vm() {
    local base_name="${1:-}" template_id="${2:-}"
    [[ -n "$base_name" && -n "$template_id" ]] || usage

    local cores="${3:-$VM_CORES}" memory="${4:-$VM_MEMORY}" disk="${5:-$VM_DISK}"
    local next_id; next_id=$(next_vmid)
    local full_vm_name="${CLOUD_NODE_PREFIX}-${base_name}-${next_id}"
    local snippet="/var/lib/vz/snippets/user-data-${next_id}.yml"

    echo "[+] Provisioning VM $next_id ($full_vm_name) from template $template_id..."
    qm clone "$template_id" "$next_id" --name "$full_vm_name" --full 0
    qm set "$next_id" --cores "$cores" --memory "$memory"
    qm resize "$next_id" scsi0 "${disk}G"

    mkdir -p /var/lib/vz/snippets
    cat <<EOF > "$snippet"
#cloud-config
user: $DEFAULT_USER
password: $DEFAULT_PASS
chpasswd: { expire: False }
ssh_pwauth: True
hostname: ${full_vm_name}
fqdn: ${full_vm_name}.local
runcmd:
  - curl -sSL ${RAW_BOOTSTRAP_URL} | bash -s -- --unattended
EOF

    qm set "$next_id" --cicustom "user=local:snippets/user-data-${next_id}.yml"
    qm start "$next_id"
    echo "[+] VM $full_vm_name ($next_id) booted and initializing."
}

destroy_asset() {
    local target_id="${1:-}"; [[ -n "$target_id" ]] || usage
    vm_exists_id "$target_id" || { echo "ERROR: Asset ID $target_id non-existent."; exit 1; }

    local vm_name
    vm_name=$(qm config "$target_id" | awk '/^name:/ {print $2}')

    echo "[-] Destroying asset $target_id ($vm_name)..."
    qm stop "$target_id" >/dev/null 2>&1 || true
    while qm status "$target_id" | grep -q running; do sleep 1; done

    if [[ -n "$vm_name" ]]; then
        clean_tailscale_node "$vm_name"
    fi

    rm -f "/var/lib/vz/snippets/user-data-${target_id}.yml"
    qm destroy "$target_id" --purge 1
    echo "[-] Asset $target_id completely removed."
}

case "$ACTION" in
    setup)           setup_environment ;;
    create-template) create_template "$@" ;;
    create-vm)       create_vm "$@" ;;
    destroy)         destroy_asset "$@" ;;
    *)               usage ;;
esac