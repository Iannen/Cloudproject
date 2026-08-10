PROVISIONING & BOOTSTRAP LAYER: STATUS & ARCHITECTURE
=====================================================

This document outlines the temporary, early-stage provisioning layer for the cloud
project. It describes how the system takes a raw Ubuntu cloud image, builds a
Proxmox template, clones virtual machines, and boots them into a minimal, self-
configuring etcd single-node mesh connected via Tailscale.

--------------------------------------------------------------------------------
HIGH-LEVEL ARCHITECTURE FLOW
--------------------------------------------------------------------------------

1. Host Asset Server: A transient Python HTTP server hosts local scripts and
   tarball assets directly on the Proxmox VE hypervisor.
2. Template Capture: Proxmox downloads a standard Ubuntu Cloud-Init image,
   injects an automated provisioning workflow, executes system-wide base software
   configurations, and converts the machine into an immutable blueprint.
3. VM Spawning: The template is cloned. Cloud-Init dynamically applies the
   correct virtual machine hostname and target credentials.
4. Autonomous Infrastructure Boot: The cloned instance executes local shell
   scripts that register the machine with Tailscale, dynamically write an internal
   environment state layout, and pull down the Core Docker Compose stack (etcd +
   custom cloud-controller).

--------------------------------------------------------------------------------
COMPONENTS SUMMARY
--------------------------------------------------------------------------------

1. The Proxmox CLI Management Utility (vms)
   - Location: Installed locally as a system binary shortcut on the Proxmox
     host (accessible via 'which vms').
   - Responsibilities: Managing the python asset server lifecycle, compiling the
     asset deployment tarball, programmatic VM template baking, VM cloning, and
     immediate disk/resource cleanup orchestration.

2. The Node Bootstrap Script Engine (bootstrap/)
   - Location: /root/cloudproject/bootstrap on the hypervisor; deployed to
     /root/bootstrap inside target VMs.
   - Key Manifest:
     * bootstrap.sh: The master router. Accepts flag arguments (--template,
       --attended, --unattended) to delegate sub-script operational phases.
     * create-env.sh: Exposes utility functions (set_env_var, get_env_var) to
       securely generate and update /root/bootstrap/.env. Explicitly handles
       hostnamectl mapping.
     * install-software.sh: EXECUTED STRICTLY DURING THE TEMPLATE PHASE. Handles
       system upgrades, apt mirror packages, Docker CE engines, and core Tailscale
       service installations.
     * join-tailscale.sh: Connects the node interface securely back to the target
       Tailnet mesh using provided ephemeral authentication keys.
     * join-etcd-cluster.sh: Collects live runtime internal network properties
       (Tailscale IP mapping) to output static configs, starts the background
       application worker stack (docker compose up -d), and awaits the master
       controller election.
     * docker-compose.yml: Defines standard infrastructure configurations for the
       isolated local etcd key-value engine and the kaffannen/cloud-controller image.

--------------------------------------------------------------------------------
KNOWN CONSTRAINTS & TECHNICAL DEBT (EARLY DEV PHASE)
--------------------------------------------------------------------------------

While the framework is fully operational and capable of reliably initializing new
system environments, the following behaviors are accepted engineering trade-offs
for the current phase of development:

- Plaintext Over-the-Wire Network Delivery: The hypervisor host-level file
  server operates via plain HTTP (python3 -m http.server). Credentials (such as
  Tailscale authentication tokens) are sent via plaintext variables inside Cloud-
  Init workflows.
  * Mitigation for future: Transition to an encrypted HTTPS transport mechanism
    or direct hypervisor-to-guest execution loops.

- Synchronous Cloud-Init Race Conditions: Custom scripts execute immediately
  inside Cloud-Init's runcmd pipeline. This can occasionally cause minor race
  conditions with systemd metadata mapping or hostname changes if network
  initialization loops stall.
  * Mitigation for future: Encapsulate the execution flow inside a localized
    custom systemd target service running post cloud-init.target.

- Hardcoded Standalone Cluster States: The join-etcd-cluster.sh component
  currently hardcodes ETCD_INITIAL_CLUSTER_STATE="new". This functions optimally
  for standalone nodes, but will disrupt consensus if cloned instances attempt
  to join an active scaling cluster.

- Brittle CLI Stream Parsing: Hypervisor ID assignment relies on basic line and
  column processing out of Proxmox stdout streams (pvesh). If output schemas
  mutate or columns drift, ID counters may fail.
  * Mitigation for future: Implement deterministic, strict JSON parsing
    pipelines (jq).

--------------------------------------------------------------------------------
HOW TO OPERATE (QUICK REFERENCE)
--------------------------------------------------------------------------------

Start Host Infrastructure:
  vms server-up

Build or Update Base Blueprint:
  vms create-template <template_name>

Instantiate Node:
  vms create-vm <vm_name> <template_vmid>

Complete Destruction:
  vms destroy <vmid>
