import base64
import json
import urllib.request
import time

from config import ETCD

ETCD_TIMEOUT = 12


def encode(s: str) -> str:
    return base64.b64encode(s.encode()).decode()


def decode(s: str) -> str:
    return base64.b64decode(s).decode()


def put(key: str, value: str, lease_id: str = None):
    payload = {
        "key": encode(key),
        "value": encode(value),
    }

    if lease_id:
        payload["lease"] = lease_id

    req = urllib.request.Request(
        f"{ETCD}/v3/kv/put",
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
        method="POST"
    )

    try:
        urllib.request.urlopen(req, timeout=ETCD_TIMEOUT).read()
    except Exception as e:
        print(f"[etcd-put-error] Failed key={key}: {e}")
        raise


def get(key: str):
    payload = json.dumps({
        "key": encode(key)
    }).encode()

    req = urllib.request.Request(
        f"{ETCD}/v3/kv/range",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST"
    )

    try:
        response = urllib.request.urlopen(
            req,
            timeout=ETCD_TIMEOUT
        ).read()

        data = json.loads(response)

        if "kvs" not in data or not data["kvs"]:
            return None

        return decode(data["kvs"][0]["value"])

    except Exception as e:
        print(f"[etcd-get-error] Failed key={key}: {e}")
        return None


def create_lease(ttl_seconds: int) -> str:
    payload = json.dumps({
        "ID": "0",
        "TTL": str(ttl_seconds)
    }).encode()

    req = urllib.request.Request(
        f"{ETCD}/v3/lease/grant",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST"
    )

    try:
        res = json.loads(
            urllib.request.urlopen(
                req,
                timeout=ETCD_TIMEOUT
            ).read()
        )

        return res["ID"]

    except Exception as e:
        print(
            f"[etcd-lease-error] "
            f"Grant failed (TTL={ttl_seconds}): {e}"
        )
        raise


def keep_alive_lease(lease_id: str):
    payload = json.dumps({
        "ID": lease_id
    }).encode()

    req = urllib.request.Request(
        f"{ETCD}/v3/lease/keepalive",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST"
    )

    try:
        urllib.request.urlopen(
            req,
            timeout=ETCD_TIMEOUT
        ).read()

    except Exception as e:
        print(
            f"Failed to keep alive lease "
            f"{lease_id}: {e}"
        )


def try_acquire_whip(
    key: str,
    value: str,
    lease_id: str
) -> bool:

    key_b64 = encode(key)

    payload = {
        "compare": [{
            "result": "EQUAL",
            "target": "VERSION",
            "key": key_b64,
            "version": "0"
        }],
        "success": [{
            "request_put": {
                "key": key_b64,
                "value": encode(value),
                "lease": lease_id
            }
        }]
    }

    req = urllib.request.Request(
        f"{ETCD}/v3/kv/txn",
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
        method="POST"
    )

    try:
        res = json.loads(
            urllib.request.urlopen(
                req,
                timeout=ETCD_TIMEOUT
            ).read()
        )

        return res.get("succeeded", False)

    except Exception as e:
        print(f"[etcd-txn-error] Transaction failed: {e}")
        return False


def list_members() -> list:
    req = urllib.request.Request(
        f"{ETCD}/v3/cluster/member/list",
        data=json.dumps({}).encode(),
        headers={"Content-Type": "application/json"},
        method="POST"
    )

    res = json.loads(
        urllib.request.urlopen(
            req,
            timeout=ETCD_TIMEOUT
        ).read()
    )

    return res.get("members", [])


#
# Existing voter add
#
def add_etcd_member(
    peer_name: str,
    peer_ip: str
) -> dict:

    target_peer_url = f"http://{peer_ip}:2380"

    payload = json.dumps({
        "peerURLs": [target_peer_url]
    }).encode()

    req = urllib.request.Request(
        f"{ETCD}/v3/cluster/member/add",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST"
    )

    try:
        res = urllib.request.urlopen(
            req,
            timeout=ETCD_TIMEOUT
        ).read()

        return json.loads(res.decode())

    except Exception as e:
        print(
            f"[etcd-member-add-error] "
            f"Failed node={peer_name} ip={peer_ip}: {e}"
        )
        raise


#
# New learner add
#
def add_etcd_learner(
    peer_name: str,
    peer_ip: str
) -> dict:

    target_peer_url = f"http://{peer_ip}:2380"

    payload = json.dumps({
        "peerURLs": [target_peer_url],
        "isLearner": True
    }).encode()

    req = urllib.request.Request(
        f"{ETCD}/v3/cluster/member/add",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST"
    )

    try:
        res = urllib.request.urlopen(
            req,
            timeout=ETCD_TIMEOUT
        ).read()

        data = json.loads(res.decode())

        print(
            f"[etcd] learner added "
            f"{peer_name}"
        )

        return data

    except Exception as e:
        print(
            f"[etcd-learner-add-error] "
            f"Failed node={peer_name} ip={peer_ip}: {e}"
        )
        raise


def promote_etcd_member(
    member_id: str
):
    payload = json.dumps({
        "ID": str(member_id)
    }).encode()

    req = urllib.request.Request(
        f"{ETCD}/v3/cluster/member/promote",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST"
    )

    try:
        urllib.request.urlopen(
            req,
            timeout=ETCD_TIMEOUT
        ).read()

        print(
            f"[etcd] promoted member "
            f"{member_id}"
        )

    except Exception as e:
        print(
            f"[etcd-promote-error] "
            f"{member_id}: {e}"
        )
        raise


def remove_etcd_member(
    member_id: str
):
    payload = json.dumps({
        "ID": str(member_id)
    }).encode()

    req = urllib.request.Request(
        f"{ETCD}/v3/cluster/member/remove",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST"
    )

    try:
        urllib.request.urlopen(
            req,
            timeout=ETCD_TIMEOUT
        ).read()

        print(
            f"[etcd] removed member "
            f"{member_id}"
        )

    except Exception as e:
        print(
            f"[etcd-remove-error] "
            f"{member_id}: {e}"
        )
        raise


def find_member_by_peer_url(
    peer_ip: str
):
    target = f"http://{peer_ip}:2380"

    members = list_members()

    for m in members:
        for url in m.get("peerURLs", []):
            if url == target:
                return m

    return None
