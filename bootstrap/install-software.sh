#!/usr/bin/env bash
set -euo pipefail

echo "System update"

apt-get update
apt-get -y upgrade

echo "Base dependencies"

apt-get install -y \
    ca-certificates \
    curl \
    gnupg \
    lsb-release \
    jq

echo "Docker"

install -m 0755 -d /etc/apt/keyrings

if [ ! -f /etc/apt/keyrings/docker.gpg ]; then
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
        | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
fi

chmod a+r /etc/apt/keyrings/docker.gpg

cat >/etc/apt/sources.list.d/docker.list <<EOF
deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable
EOF

apt-get update

apt-get install -y \
    docker-ce \
    docker-ce-cli \
    containerd.io \
    docker-buildx-plugin \
    docker-compose-plugin

systemctl enable --now docker

echo "Tailscale"

curl -fsSL https://tailscale.com/install.sh | sh

systemctl enable tailscaled

echo "Storage / time sync utilities"

apt-get install -y \
    chrony \
    lvm2 \
    nvme-cli \
    smartmontools

systemctl enable --now chrony

echo "Installed versions"

docker --version
tailscale version
