import json
import subprocess
import threading
import time
import urllib.request
import uuid

import etcd
from config import (
    HOSTNAME,
    CLUSTER_ID_KEY,
    CLOUD_STATUS_KEY,
    WHIP_LEADER_KEY,
    CLOUD_STATUS_NOT_INITIALIZED,
)

IS_WHIP = False
CURRENT_LEASE_ID = None


def wait_for_etcd():
    while True:
        try:
            etcd.list_members()
            break
        except Exception as e:
            print(f"Waiting for local etcd... {e}")
            time.sleep(2)


def initialize_cloud_metadata():
    if etcd.get(CLUSTER_ID_KEY) is None:
        etcd.put(CLUSTER_ID_KEY, str(uuid.uuid4()))

    if etcd.get(CLOUD_STATUS_KEY) is None:
        etcd.put(CLOUD_STATUS_KEY, CLOUD_STATUS_NOT_INITIALIZED)


def promote_to_whip():
    global IS_WHIP, CURRENT_LEASE_ID

    if IS_WHIP:
        return True

    try:
        print(f"[{HOSTNAME}] Promotion requested. Creating initialization lease...")

        lease_id = etcd.create_lease(30)
        success = etcd.try_acquire_whip(
            WHIP_LEADER_KEY,
            HOSTNAME,
            lease_id,
        )

        if success:
            IS_WHIP = True
            CURRENT_LEASE_ID = lease_id
            print(f"[{HOSTNAME}] Successfully elected as cluster Whip!")
            return True

    except Exception as e:
        print(f"[{HOSTNAME}] Error trying to claim Whip role: {e}")

    return False


def discover_tailscale_peers() -> list:
    try:
        result = subprocess.run(
            ["tailscale", "status", "--json"],
            capture_output=True,
            text=True,
            check=True,
        )

        data = json.loads(result.stdout)
        peers = []

        if "Peer" in data:
            for _, peer_info in data["Peer"].items():
                dns_name = peer_info.get("DNSName", "").lower()

                if dns_name:
                    short_name = dns_name.split(".")[0]

                    my_dns = data.get("Self", {}).get("DNSName", "").lower()
                    my_short_name = (
                        my_dns.split(".")[0]
                        if my_dns
                        else HOSTNAME.lower()
                    )

                    if (
                        short_name.startswith("kaffcloud-")
                        and short_name != my_short_name
                    ):
                        ips = peer_info.get("TailscaleIPs", [])

                        if ips:
                            peers.append(
                                {
                                    "hostname": short_name,
                                    "ip": ips[0],
                                }
                            )

        return peers

    except Exception as e:
        print(f"[{HOSTNAME}] Error executing tailscale discovery: {e}")
        return []


def wait_for_member_joined(
    recruit_hostname: str,
    timeout_seconds: int = 120,
) -> bool:
    print(
        f"[{HOSTNAME}] Waiting for {recruit_hostname} to join etcd cluster..."
    )

    start = time.time()

    while time.time() - start < timeout_seconds:
        try:
            members = etcd.list_members()

            for m in members:
                if (
                    m.get("name") == recruit_hostname
                    and m.get("clientURLs")
                ):
                    print(
                        f"[{HOSTNAME}] {recruit_hostname} successfully joined the cluster!"
                    )
                    return True

        except Exception as e:
            if "503" in str(e):
                print(
                    f"[{HOSTNAME}] etcd 503 (expected during join) - still waiting..."
                )
            else:
                print(f"[{HOSTNAME}] Error polling members: {e}")

        time.sleep(5)

    print(f"[{HOSTNAME}] Timeout waiting for {recruit_hostname} to join.")
    return False


def run_recruitment_batch():
    global CURRENT_LEASE_ID

    print(f"[{HOSTNAME}] [Recruitment] Scanning Tailscale peer configurations...")

    peers = discover_tailscale_peers()

    print(
        f"[{HOSTNAME}] [Recruitment] Found {len(peers)} matching active kaffcloud mesh nodes."
    )

    if not peers:
        return

    my_cluster_id = etcd.get(CLUSTER_ID_KEY)

    if my_cluster_id is None:
        print(f"[{HOSTNAME}] Cannot recruit - missing cluster_id")
        return
    else:
        print(
            f"[{HOSTNAME}] [Recruitment] Leader cluster_id = {my_cluster_id}"
        )

    recruits = []

    for peer in peers:
        try:
            url = f"http://{peer['ip']}:8080/status"

            print(
                f"[{HOSTNAME}] [Recruitment] Checking {peer['hostname']} at {url}"
            )

            req = urllib.request.Request(url)

            res = json.loads(
                urllib.request.urlopen(req, timeout=8)
                .read()
                .decode()
            )

            peer_cluster = res.get("cluster", {})
            p_id = peer_cluster.get("cluster_id")
            p_init = peer_cluster.get("initialized", False)

            print(
                f"[{HOSTNAME}] [Recruitment] {peer['hostname']} responded: "
                f"cluster_id={p_id}, initialized={p_init}"
            )

            if p_id != my_cluster_id and not p_init:
                print(
                    f"[{HOSTNAME}] [Recruitment] >>> TARGET IDENTIFIED: {peer['hostname']} <<<"
                )
                recruits.append(peer)
            else:
                print(
                    f"[{HOSTNAME}] [Recruitment] Skipping {peer['hostname']}"
                )

        except Exception as e:
            print(
                f"[{HOSTNAME}] [Recruitment] Failed to check "
                f"{peer['hostname']} at {url}: "
                f"{type(e).__name__} - {e}"
            )
            continue

    if not recruits:
        print(
            f"[{HOSTNAME}] [Recruitment] No suitable uninitialized nodes found this sweep - aborting."
        )
        return

    print(f"[{HOSTNAME}] Found {len(recruits)} nodes ready for recruitment.")

    try:
        print(f"[{HOSTNAME}] Extending lease to 600s for recruitment...")

        massive_lease = etcd.create_lease(600)

        etcd.put(
            WHIP_LEADER_KEY,
            HOSTNAME,
            lease_id=massive_lease,
        )

        CURRENT_LEASE_ID = massive_lease
        my_local_ip = "127.0.0.1"
#######################################################################################################################
        for recruit in recruits:
            print(f"[{HOSTNAME}] Annexing {recruit['hostname']} as a Learner...")
            recruit_member_id = None

            try:
                # Stage 1: Add member as non-voting learner via etcd HTTP API
                response_data = etcd.add_etcd_learner(recruit["hostname"], recruit["ip"])
                current_members = response_data.get("members", [])

                # Resolve the allocated member ID and extract peer strings
                cluster_tokens = []
                target_peer_url = f"http://{recruit['ip']}:2380"

                for m in current_members:
                    m_name = m.get("name") if m.get("name") else recruit["hostname"]
                    p_urls = m.get("peerURLs", [])

                    if p_urls:
                        cluster_tokens.append(f"{m_name}={p_urls[0]}")
                        if p_urls[0] == target_peer_url:
                            recruit_member_id = m.get("ID")

                    if m_name == HOSTNAME and p_urls:
                        my_local_ip = p_urls[0].replace("http://", "").split(":")[0]

                if not recruit_member_id:
                    print(f"[{HOSTNAME}] Could not look up member ID for staged learner. Skipping.")
                    continue

                payload = json.dumps({
                    "whip_node_name": HOSTNAME,
                    "whip_etcd_peer_url": f"http://{my_local_ip}:2380",
                    "etcd_initial_cluster": ",".join(cluster_tokens),
                    "cluster_id": my_cluster_id,
                    "assigned_ip": recruit["ip"],
                }).encode()

                print(f"[{HOSTNAME}] Transmitting payload specs to {recruit['hostname']}")

                # Stage 2: POST configuration to follower (Expect early 200 OK from Docker status check)
                req = urllib.request.Request(
                    f"http://{recruit['ip']}:8080/assimilate",
                    data=payload,
                    headers={"Content-Type": "application/json"},
                    method="POST"
                )
                urllib.request.urlopen(req, timeout=40).read()

                # Stage 3: Validate sync status from the leader perspective
                is_joined = wait_for_member_joined(recruit["hostname"], timeout_seconds=60)

                if is_joined:
                    # Stage 4: Promote candidate
                    print(f"[{HOSTNAME}] Promoting {recruit['hostname']} ({recruit_member_id}) to Voter...")
                    etcd.promote_etcd_member(recruit_member_id)
                    print(f"[{HOSTNAME}] Successfully integrated {recruit['hostname']}.")
                else:
                    raise RuntimeError("Follower container started but failed to sync Raft logs in time.")

            except Exception as e:
                print(f"[{HOSTNAME}] [Swipe] Issue encountered on node {recruit['hostname']}: {e}")
                if recruit_member_id:
                    print(f"[{HOSTNAME}] Evicting broken learner configuration {recruit_member_id}...")
                    try:
                        etcd.remove_etcd_member(recruit_member_id)
                    except Exception as ex:
                        print(f"[{HOSTNAME}] Critical: Failed member cleanup for {recruit_member_id}: {ex}")
                print(f"[{HOSTNAME}] Moving past to next candidate...")
                continue
####################################################################################################################

    except Exception as e:
        print(f"[{HOSTNAME}] Recruitment error: {e}")

    finally:
        print(
            f"[{HOSTNAME}] Recruitment batch finished. Restoring normal lease."
        )

        try:
            normal_lease = etcd.create_lease(30)

            etcd.put(
                WHIP_LEADER_KEY,
                HOSTNAME,
                lease_id=normal_lease,
            )

            CURRENT_LEASE_ID = normal_lease

        except Exception as e:
            print(f"Failed resetting lease: {e}")


def loop_runner():
    global CURRENT_LEASE_ID

    print(f"[{HOSTNAME}] Main Supervisor Core loop initialized.")

    while True:
        time.sleep(15)

        if IS_WHIP:
            try:
                if CURRENT_LEASE_ID is None:
                    CURRENT_LEASE_ID = etcd.create_lease(30)

                    etcd.put(
                        WHIP_LEADER_KEY,
                        HOSTNAME,
                        lease_id=CURRENT_LEASE_ID,
                    )

                etcd.keep_alive_lease(CURRENT_LEASE_ID)
                run_recruitment_batch()

            except Exception as e:
                print(f"Whip background loop tracking error: {e}")


def start_background_threads():
    t = threading.Thread(
        target=loop_runner,
        daemon=True,
    )
    t.start()
