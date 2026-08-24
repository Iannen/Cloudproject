I. Long term goals (ignore)

II. Medium term goals (investigatory)

[x] Endure that cluster members will attempt to become leader when I take down leader node (to simulate power outage and such)

[ ] Add etcd status endpoint, for easy and comprehensive diagnostics of etcd related issues


III. Immediate Goals (consider these first)

[ ] Refactor MemberRole leadership handling to use declarative assignment reconciliation
    [ ] Decouple ClaimLeader from direct role invocation
        - Update MemberRole so winning ClaimLeader writes a leader assignment definition to etcd rather than calling registry.Start directly
    [ ] Normalize LeaderRole management inside reconcile()
        - Ensure MemberRole relies solely on etcd assignment watch events to launch or terminate LeaderRole
        - Allow reconcile() to handle LeaderRole teardown naturally when assignments drop or re-bind to another node

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
        3. bug -> remaining 2 nodes both log '2026/08/24 10:51:31 [Member] Session terminated: session lease expired'
