=== vms ===
#!/usr/bin/env bash
set -euo pipefail

################################################################################
# CONFIGS
################################################################################

PROJECT_ROOT="/root/cloudproject"
BOOTSTRAP_DIR="$PROJECT_ROOT/bootstrap"
CONTROLLER_DIR="$PROJECT_ROOT/controller"

# Storage & Network
STORAGE="local-zfs"
BRIDGE="vmbr0"

# Asset Server & Proxmox Configuration
PROXMOX_IP="192.168.50.246"
ASSET_PORT="8000"

# Default Templates Sizing
TEMPLATE_CORES=2
TEMPLATE_MEMORY=4096
TEMPLATE_DISK=20
DEFAULT_IMAGE_URL="https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img"

# Default VM Allocation overrides (if not explicitly passed)
VM_CORES=4
VM_MEMORY=3072
VM_DISK=100

# Cloud-Init Credentials
DEFAULT_USER="user"
DEFAULT_PASS="pass"

################################################################################
# HELP / USAGE
################################################################################

usage() {
cat <<EOF
Usage:

# Server management
$0 server-up
$0 server-down

# Template / VM Architecture
$0 create-template <template_name> [url] [cores] [memory] [disk]
$0 create-vm <vm_name> <template_vmid> [cores] [memory] [disk]
$0 destroy <vmid>


EOF
exit 1
}

################################################################################
# ARGUMENT ROUTER LOGIC HELPERS
################################################################################

ACTION="${1:-}"
[[ -n "$ACTION" ]] || usage
shift

vm_exists_id() {
    qm status "$1" >/dev/null 2>&1
}

next_vmid() {
    pvesh get /cluster/nextid
}

next_template_vmid() {
    local max_template_id
    max_template_id=$(pvesh get /cluster/resources --type vm --output vmid | awk 'NR>1 {print $1}' | awk '$1 >= 9000 {print $1}' | sort -n | tail -n1)
      
    if [[ -z "$max_template_id" ]]; then
        echo "9000"
    else
        echo $((max_template_id + 1))
    fi
}

asset_server_running() {
    ss -tlnp | grep -q ":${ASSET_PORT} "
}

require_asset_server() {
    if ! asset_server_running; then
        echo "ERROR: Asset server is down! Please run '$0 server-up' first."
        exit 1
    fi
}

################################################################################
# SERVER RELATED FUNCTIONS
################################################################################

server_up() {
    echo "=================================================="
    echo "Starting / Refreshing Server Infrastructure"
    echo "=================================================="
      
    if asset_server_running; then
        pkill -f "python3 -m http.server --directory ${BOOTSTRAP_DIR} ${ASSET_PORT}" || true
        sleep 1
    fi

    echo "Compiling clean bootstrap archive tarball..."
    rm -f "$BOOTSTRAP_DIR/bootstrap.tar.gz"
    tar -czf /tmp/bootstrap.tar.gz -C "$BOOTSTRAP_DIR" .
    mv /tmp/bootstrap.tar.gz "$BOOTSTRAP_DIR/bootstrap.tar.gz"

    echo "Starting asset server on port ${ASSET_PORT}..."
    nohup python3 -m http.server --directory "$BOOTSTRAP_DIR" "$ASSET_PORT" >/dev/null 2>&1 &
    sleep 1
    echo "Bootstrap infrastructure and completely fresh assets up and ready."
}

server_down() {
    echo "=================================================="
    echo "Tearing Down Server Infrastructure"
    echo "=================================================="
    if asset_server_running; then
        pkill -f "python3 -m http.server --directory ${BOOTSTRAP_DIR} ${ASSET_PORT}" || true
        sleep 1
    fi
    rm -f "$BOOTSTRAP_DIR/bootstrap.tar.gz"
    echo "Server artifacts cleaned and running daemon terminated."
}

################################################################################
# TEMPLATE MANAGEMENT
################################################################################

create_template() {
    require_asset_server

    local template_name="${1:-}"
    [[ -n "$template_name" ]] || usage
    shift

    local img_url="${1:-$DEFAULT_IMAGE_URL}"
    local cores="${2:-$TEMPLATE_CORES}"
    local memory="${3:-$TEMPLATE_MEMORY}"
    local disk="${4:-$TEMPLATE_DISK}"

    echo ""
    read -rp "Please input ts key for unattended bootstrap: " TS_AUTH_KEY
    echo ""
    if [[ -z "$TS_AUTH_KEY" ]]; then
        echo "ERROR: Tailscale authentication key cannot be empty."
        exit 1
    fi

    local vmid
    vmid=$(next_template_vmid)
    local full_template_name="${template_name}-${vmid}"
    local tmp_img="/tmp/downloaded_cloud_base_${vmid}.img"

    echo "Downloading cloud image from $img_url..."
    curl -L -o "$tmp_img" "$img_url"

    echo "Initializing base VM structure for Template ID: $vmid ($full_template_name)"
    qm create "$vmid" \
        --name "$full_template_name" \
        --memory "$memory" \
        --cores "$cores" \
        --cpu host \
        --machine q35 \
        --bios ovmf \
        --net0 virtio,bridge="$BRIDGE" \
        --scsihw virtio-scsi-single \
        --serial0 socket \
        --vga serial0

    echo "Importing disk architecture..."
    qm importdisk "$vmid" "$tmp_img" "$STORAGE"
    qm set "$vmid" --scsi0 "$STORAGE:vm-$vmid-disk-0"
      
    local snippet_dir="/var/lib/vz/snippets"
    mkdir -p "$snippet_dir"
    local snippet_file="${snippet_dir}/user-data-${vmid}.yml"

    echo "Generating default automated template Cloud-Init YAML configuration..."
    cat <<EOF > "$snippet_file"
#cloud-config
user: $DEFAULT_USER
password: $DEFAULT_PASS
chpasswd: { expire: False }
ssh_pwauth: True

# Set hostname for the template
hostname: ${full_template_name}
fqdn: ${full_template_name}.local

power_state:
  delay: "now"
  mode: "poweroff"

runcmd:
  - curl -sSL http://${PROXMOX_IP}:${ASSET_PORT}/bootstrap.sh | bash -s -- --template --key "${TS_AUTH_KEY}"
EOF

    qm set "$vmid" \
        --efidisk0 "$STORAGE:1" \
        --ide2 "$STORAGE:cloudinit" \
        --boot order=scsi0 \
        --ciuser "$DEFAULT_USER" \
        --cipassword "$DEFAULT_PASS" \
        --ipconfig0 ip=dhcp \
        --cicustom "user=local:snippets/user-data-${vmid}.yml"

    echo "Resizing partition context to ${disk}G..."
    qm resize "$vmid" scsi0 "${disk}G"
    rm -f "$tmp_img"

    echo "Booting VM to let Cloud-Init execute provisioning code..."
    qm start "$vmid"

    echo "Waiting for guest OS initialization to trigger automated poweroff..."
    while qm status "$vmid" | grep -q running; do
        sleep 5
    done

    echo "Converting structural scratchpad $vmid into immutable blueprint..."
    qm template "$vmid"
      
    rm -f "$snippet_file"
    echo "Successfully generated template blueprint asset ID: $vmid"
}

################################################################################
# VM MANAGEMENT (with proper hostname)
################################################################################

create_vm() {
    require_asset_server

    local base_name="${1:-}"
    local template_id="${2:-}"
    [[ -n "$base_name" && -n "$template_id" ]] || usage

    local cores="${3:-$VM_CORES}"
    local memory="${4:-$VM_MEMORY}"
    local disk="${5:-$VM_DISK}"

    local next_id
    next_id=$(next_vmid)
    local full_vm_name="${base_name}-${next_id}"

    echo "Cloning node instance from Template $template_id into target VMID $next_id..."
    qm clone "$template_id" "$next_id" --name "$full_vm_name" --full 0

    # Update hardware specs + Proxmox name
    qm set "$next_id" --cores "$cores" --memory "$memory" --name "$full_vm_name"
    qm resize "$next_id" scsi0 "${disk}G"

    local snippet_dir="/var/lib/vz/snippets"
    mkdir -p "$snippet_dir"
    local snippet_file="${snippet_dir}/user-data-${next_id}.yml"

    echo "Generating Cloud-Init config with proper hostname..."
    cat <<EOF > "$snippet_file"
#cloud-config
user: $DEFAULT_USER
password: $DEFAULT_PASS
chpasswd: { expire: False }
ssh_pwauth: True

# Set the desired hostname early (this is the key fix)
hostname: ${full_vm_name}
fqdn: ${full_vm_name}.local

runcmd:
  - mkdir test2
  - curl -sSL http://${PROXMOX_IP}:${ASSET_PORT}/bootstrap.sh | bash -s -- --unattended
EOF

    qm set "$next_id" --cicustom "user=local:snippets/user-data-${next_id}.yml"

    echo "Booting Target Factory Instance Node: $full_vm_name ($next_id)..."
    qm start "$next_id"
    echo "VM $full_vm_name ($next_id) has been provisioned and is initializing via Cloud-Init."
}

################################################################################
# UNIFIED COMPACT DESTROY ENGINE
################################################################################

destroy_asset() {
    local target_id="${1:-}"
    [[ -n "$target_id" ]] || usage

    vm_exists_id "$target_id" || {
        echo "ERROR: Asset ID $target_id context not found in hypervisor registry mapping."
        exit 1
    }

    echo "Enforcing immediate power termination for context ID: $target_id"
    qm stop "$target_id" >/dev/null 2>&1 || true

    while qm status "$target_id" | grep -q running; do
        sleep 1
    done

    echo "Sweeping leftover cloud-init programmatic properties..."
    rm -f "/var/lib/vz/snippets/user-data-${target_id}.yml"

    echo "Purging asset allocation resources from host pools..."
    qm destroy "$target_id" --purge 1
    echo "Destroyed and wiped completely: Asset ID $target_id"
}

################################################################################
# ACTION ROUTER INTERFACE
################################################################################

case "$ACTION" in
    server-up)       server_up ;;
    server-down)     server_down ;;
    create-template) create_template "$@" ;;
    create-vm)       create_vm "$@" ;;
    destroy)         destroy_asset "$@" ;;
    *)               usage ;;
esac