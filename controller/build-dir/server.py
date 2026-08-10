import json
import os
import subprocess
import time

from http.server import HTTPServer, BaseHTTPRequestHandler

import etcd
import whip_role

from config import (
    LISTEN_PORT,
    HOSTNAME,
    CLUSTER_ID_KEY,
    CLOUD_STATUS_KEY,
)


def assimilate_into_cluster(data):
    """
    Convert this node from a singleton into a member of an existing cluster.

    This call only returns once the local etcd member is serving client
    requests successfully.
    """

    print(f"[{HOSTNAME}] Beginning assimilation...")

    bootstrap_dir = f"{os.environ.get('HOME', '/root')}/bootstrap"
    env_file = f"{bootstrap_dir}/.env"

    ts_ip = data.get("assigned_ip")
    if not ts_ip:
        raise KeyError("Assimilation payload missing critical 'assigned_ip' descriptor.")
    print(f"[{HOSTNAME}]:  Tailscale IP: {ts_ip}")

    #
    # Rewrite cluster configuration to hook into existing mesh network infrastructure.
    #
    with open(env_file, "w") as f:
        f.write(f"HOSTNAME={HOSTNAME}\n")
        f.write(f"TAILSCALE_IP={ts_ip}\n")
        f.write(f"ETCD_NAME={HOSTNAME}\n")
        f.write(
            f"ETCD_INITIAL_CLUSTER="
            f"{data['etcd_initial_cluster']}\n"
        )
        f.write("ETCD_INITIAL_CLUSTER_STATE=existing\n")

    print(f"[{HOSTNAME}] Updated bootstrap .env")

    #
    # Replace singleton etcd engine with the joining etcd engine instance.
    # Wiping volumes is crucial to prevent split-brain runtime metadata conflicts.
    #
    print(f"[{HOSTNAME}] Restarting etcd...")

    subprocess.run(
        [
            "docker",
            "compose",
            "down",
            "etcd",
            "--volumes",
            "--remove-orphans",
        ],
        cwd=bootstrap_dir,
        check=True,
    )

    subprocess.run(
        [
            "docker",
            "compose",
            "up",
            "-d",
            "etcd",
        ],
        cwd=bootstrap_dir,
        check=True,
    )

    print(f"[{HOSTNAME}] Monitoring docker container execution state...")

    deadline = time.time() + 30

    while time.time() < deadline:
        try:
            result = subprocess.run(
                ["docker", "compose", "ps", "etcd", "--format", "json"],
                cwd=bootstrap_dir,
                capture_output=True,
                text=True,
                check=True
            )

            status_data = json.loads(result.stdout)
            if isinstance(status_data, list):
                status_data = status_data[0] if status_data else {}

            state = status_data.get("State", "").lower()

            if state == "running":
                print(f"[{HOSTNAME}] Etcd container verified running in Docker. Signalling leader.")
                return

        except Exception as e:
            print(f"[{HOSTNAME}] Error checking container status: {e}")

        time.sleep(1)

    raise RuntimeError("Timed out waiting for Docker etcd container to start.")


class Handler(BaseHTTPRequestHandler):

    def do_GET(self):

        if self.path == "/status":

            c_id = etcd.get(CLUSTER_ID_KEY)
            c_status = etcd.get(CLOUD_STATUS_KEY)

            response = {
                "hostname": HOSTNAME,
                "cluster": {
                    "cluster_id": (
                        c_id if c_id else "unknown"
                    ),
                    "initialized": (
                        c_status != "not_initialized"
                        and c_status is not None
                    ),
                },
            }

            self.send_response(200)
            self.send_header(
                "Content-Type",
                "application/json",
            )
            self.end_headers()

            self.wfile.write(
                json.dumps(response).encode()
            )
            return

        self.send_response(404)
        self.end_headers()

    def do_POST(self):

        if self.path == "/make-whip":

            success = whip_role.promote_to_whip()

            self.send_response(200)
            self.send_header(
                "Content-Type",
                "application/json",
            )
            self.end_headers()

            self.wfile.write(
                json.dumps({
                    "status":
                        "whip_accepted"
                        if success
                        else "failed"
                }).encode()
            )
            return

        if self.path == "/assimilate":

            try:
                length = int(
                    self.headers["Content-Length"]
                )

                data = json.loads(
                    self.rfile.read(length).decode()
                )

                assimilate_into_cluster(data)

                self.send_response(200)
                self.send_header(
                    "Content-Type",
                    "application/json",
                )
                self.end_headers()

                self.wfile.write(
                    json.dumps({
                        "status": "done"
                    }).encode()
                )

            except Exception as e:
                print(
                    f"[{HOSTNAME}] "
                    f"Assimilation failed: {e}"
                )

                self.send_response(500)
                self.send_header(
                    "Content-Type",
                    "application/json",
                )
                self.end_headers()

                self.wfile.write(
                    json.dumps({
                        "status": "failed",
                        "error": str(e),
                    }).encode()
                )
            return

        self.send_response(404)
        self.end_headers()

    def log_message(self, format, *args):
        return


def start_http_server():

    server = HTTPServer(
        ("0.0.0.0", LISTEN_PORT),
        Handler,
    )

    print(
        f"HTTP Server online "
        f"on port {LISTEN_PORT}..."
    )

    server.serve_forever()
