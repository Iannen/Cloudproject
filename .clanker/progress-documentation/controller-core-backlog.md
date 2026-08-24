I. Long term goals (ignore)

[ ] Redesign role lifecycle architecture into a 3-tier hierarchy
    - Tier 1 (NodeRole): Maintained directly by main.go as host process; exempt from registry
    - Tier 2 (MemberRole): Managed directly by NodeRole via RPC/HTTP activation; exempt from registry; manages etcd session and elections
    - Tier 3 (Dynamic Roles): Workloads managed exclusively by MemberRole via Registry (e.g., leader, tailscale-manager)
    - Goal: Eliminate circular registry self-management and simplify MemberRole state recovery

II. Medium term goals (investigatory)

[ ] Add etcd status endpoint, for easy and comprehensive diagnostics of etcd related issues

III. Immediate Goals (consider these first)

[ ] Fix session lease expiration termination in MemberRole
- Refactor MemberRole.Run to wrap session creation and runSession in an outer retry loop
- Re-establish session, presence heartbeat, and event watchers upon session expiration or transient raft leader unavailability
- Ensure managed non-core assignments (any but node and member per helper) are cleanly stopped before attempting session recovery

IV. Idea bucket:

V. Bugs:
    [ ] 3 node experiment to demonstrate bug
        1. three nodes boot, one is chosen as leader via initialize endpoint. it assimilates the other 2 nodes, yielding below status
+----------------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+79 endpoint status -w table
|          ENDPOINT          |        ID        | VERSION | DB SIZE | IS LEADER | IS LEARNER | RAFT TERM | RAFT INDEX | RAFT APPLIED INDEX | ERRORS |
+----------------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
| http://100.119.155.14:2379 | e067c70e92196295 |  3.5.23 |   20 kB |      true |      false |         2 |         25 |                 25 |        |
| http://100.105.185.34:2379 | ddbd1eee81e0dba7 |  3.5.23 |   20 kB |     false |      false |         2 |         25 |                 25 |        |
|   http://100.65.82.53:2379 | 7558006d1a23d552 |  3.5.23 |   20 kB |     false |      false |         2 |         25 |                 25 |        |
+----------------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
        2. leadernode 100.119.155.14 is 'qm stopped'
        3. Root cause: when etcd Raft leader dies, remaining nodes fail to keepalive/renew their etcd session TTL during Raft election window, causing `sess.Done()` to fire ('Session terminated: session lease expired') before leadership claim can process.
