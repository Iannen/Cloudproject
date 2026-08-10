#!/usr/bin/env python3

from threading import Thread

import config
import whip_role
import server


def main():
    print("========================================")
    print("Controller starting")
    print(f"Hostname: {config.HOSTNAME}")
    print(f"ETCD_ENDPOINTS: {config.ETCD}")
    print("========================================")

    # Block until local etcd engine responds
    whip_role.wait_for_etcd()

    # Establish cluster base records
    whip_role.initialize_cloud_metadata()

    # Fire up API endpoints
    server_thread = Thread(
        target=server.start_http_server,
        daemon=True,
    )
    server_thread.start()

    # Keep main thread alive driving lease supervisor
    whip_role.loop_runner()


if __name__ == "__main__":
    main()
