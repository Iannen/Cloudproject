import os
import socket

ETCD = os.environ["ETCD_ENDPOINTS"]
HOSTNAME = socket.gethostname()
LISTEN_PORT = int(os.environ.get("CONTROLLER_PORT", "8080"))

# Etcd tracking keys
CLUSTER_ID_KEY = "/cloud/cluster_id"
CLOUD_STATUS_KEY = "/cloud/status"
WHIP_LEADER_KEY = "/cloud/whip_leader"

CLOUD_STATUS_NOT_INITIALIZED = "not_initialized"
