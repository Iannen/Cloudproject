#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="/root/cloudproject"
BOOTSTRAP_DIR="$PROJECT_ROOT/bootstrap"
STORAGE="local-zfs"
BRIDGE="vmbr0"
PROXMOX_IP="192.168.50.246"
ASSET_PORT="8000"

TEMPLATE_CORES=2; TEMPLATE_MEMORY=4096; TEMPLATE_DISK=20
VM_CORES=4; VM_MEMORY=3072; VM_DISK=100
DEFAULT_IMAGE_URL="https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img"
DEFAULT_USER="user"; DEFAULT_PASS="pass"

usage() {
    cat <<EOF
Usage:
  $0 server-up | server-down
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
    max_id=$(pvesh get /cluster/resources --type vm --output vmid | awk 'NR>1 && $1>=9000 {print $1}' | sort -n | tail -n1)
    echo "${max_id:-8999}" | awk '{print $1+1}'
}

asset_server_running() { ss -tlnp | grep -q ":${ASSET_PORT} "; }

require_asset_server() {
    asset_server_running || { echo "ERROR: Asset server is down. Run '$0 server-up' first."; exit 1; }
}

server_up() {
    echo "[+] Starting infrastructure asset server..."
    server_down >/dev/null 2>&1 || true
    
    tar -czf /tmp/bootstrap.tar.gz -C "$BOOTSTRAP_DIR" .
    mv /tmp/bootstrap.tar.gz "$BOOTSTRAP_DIR/bootstrap.tar.gz"
    nohup python3 -m http.server --directory "$BOOTSTRAP_DIR" "$ASSET_PORT" >/dev/null 2>&1 &
    sleep 1
    echo "[+] Asset server ready on port ${ASSET_PORT}."
}

server_down() {
    echo "[-] Tearing down infrastructure asset server..."
    pkill -f "python3 -m http.server --directory ${BOOTSTRAP_DIR} ${ASSET_PORT}" || true
    rm -f "$BOOTSTRAP_DIR/bootstrap.tar.gz"
}

create_template() {
    require_asset_server
    local template_name="${1:-}"; [[ -n "$template_name" ]] || usage; shift

    local img_url="${1:-$DEFAULT_IMAGE_URL}"
    local cores="${2:-$TEMPLATE_CORES}" memory="${3:-$TEMPLATE_MEMORY}" disk="${4:-$TEMPLATE_DISK}"

    read -rp "Enter Tailscale key for template: " TS_AUTH_KEY
    [[ -n "$TS_AUTH_KEY" ]] || { echo "ERROR: Key required."; exit 1; }

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
user: $DEFAULT_USER
password: $DEFAULT_PASS
chpasswd: { expire: False }
ssh_pwauth: True
hostname: ${full_name}
fqdn: ${full_name}.local
power_state: { delay: "now", mode: "poweroff" }
runcmd:
  - curl -sSL http://${PROXMOX_IP}:${ASSET_PORT}/bootstrap.sh | bash -s -- --template --key "${TS_AUTH_KEY}"
EOF

    qm set "$vmid" --efidisk0 "$STORAGE:1" --ide2 "$STORAGE:cloudinit" --boot order=scsi0 \
        --ciuser "$DEFAULT_USER" --cipassword "$DEFAULT_PASS" --ipconfig0 ip=dhcp \
        --cicustom "user=local:snippets/user-data-${vmid}.yml"
    
    qm resize "$vmid" scsi0 "${disk}G"
    rm -f "$tmp_img"

    qm start "$vmid"
    while qm status "$vmid" | grep -q running; do sleep 3; done

    qm template "$vmid"
    rm -f "$snippet"
    echo "[+] Template blueprint created successfully: $vmid"
}

create_vm() {
    require_asset_server
    local base_name="${1:-}" template_id="${2:-}"
    [[ -n "$base_name" && -n "$template_id" ]] || usage

    local cores="${3:-$VM_CORES}" memory="${4:-$VM_MEMORY}" disk="${5:-$VM_DISK}"
    local next_id; next_id=$(next_vmid)
    local full_vm_name="${base_name}-${next_id}"
    local snippet="/var/lib/vz/snippets/user-data-${next_id}.yml"

    echo "[+] Provisioning VM $next_id ($full_vm_name) from template $template_id..."
    qm clone "$template_id" "$next_id" --name "$full_vm_name" --full 0
    qm set "$next_id" --cores "$cores" --memory "$memory"
    qm resize "$next_id" scsi0 "${disk}G"

    mkdir -p /var/lib/vz/snippets
    cat <<EOF > "$snippet"
user: $DEFAULT_USER
password: $DEFAULT_PASS
chpasswd: { expire: False }
ssh_pwauth: True
hostname: ${full_vm_name}
fqdn: ${full_vm_name}.local
runcmd:
  - mkdir -p /root/test2
  - curl -sSL http://${PROXMOX_IP}:${ASSET_PORT}/bootstrap.sh | bash -s -- --unattended
EOF

    qm set "$next_id" --cicustom "user=local:snippets/user-data-${next_id}.yml"
    qm start "$next_id"
    echo "[+] VM $full_vm_name ($next_id) booted and initializing."
}

destroy_asset() {
    local target_id="${1:-}"; [[ -n "$target_id" ]] || usage
    vm_exists_id "$target_id" || { echo "ERROR: Asset ID $target_id non-existent."; exit 1; }

    echo "[-] Destroying asset $target_id..."
    qm stop "$target_id" >/dev/null 2>&1 || true
    while qm status "$target_id" | grep -q running; do sleep 1; done

    rm -f "/var/lib/vz/snippets/user-data-${target_id}.yml"
    qm destroy "$target_id" --purge 1
    echo "[-] Asset $target_id removed."
}

case "$ACTION" in
    server-up)       server_up ;;
    server-down)     server_down ;;
    create-template) create_template "$@" ;;
    create-vm)       create_vm "$@" ;;
    destroy)         destroy_asset "$@" ;;
    *)               usage ;;
esac