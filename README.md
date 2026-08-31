# Personal Cloud Controller

A lightweight, self-hosting framework for running a personal overlay cloud on top of Proxmox VE. 

It handles automated VM provisioning via Cloud-Init, wires nodes together using Tailscale, and coordinates cluster state with an `etcd`-backed Go controller engine.

---

## The 3 Pillars

1. **PVE Harness (`vms.sh`)** — Proxmox scripts for serving assets, baking Cloud-Init templates, and spinning up new VMs.
2. **Bootstrap Package (`bootstrap/`)** — Node setup "cocoon" that configures host networking, connects to Tailscale, and launches the runtime stack.
3. **Go Controller (`go-controller/`)** — Distributed engine managing node state, leader elections, and task execution via `etcd`.

---

## Quick Start

```bash
# 1. Start asset server on Proxmox host
./vms.sh server-up

# 2. Bake a VM template
./vms.sh create-template cloud-base

# 3. Provision a new node
./vms.sh create-vm node-01 <TEMPLATE_VMID>
```
### IV: Project status:
31.08.26 
One push to implement remainder of backlog will leave the system in a state where it handles node death & can recruit new tailnet members
(at least inside the proxmox vm environment, but it should translate to nodes outside of that as well)