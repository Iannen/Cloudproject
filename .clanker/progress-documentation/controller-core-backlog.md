I. Long term goals (ignore)

[ ] Redesign role lifecycle architecture into a 3-tier hierarchy
    - Tier 1 (NodeRole): Maintained directly by main.go as host process; exempt from registry
    - Tier 2 (MemberRole): Managed directly by NodeRole via RPC/HTTP activation; exempt from registry; manages etcd session and elections
    - Tier 3 (Dynamic Roles): Workloads managed exclusively by MemberRole via Registry (e.g., leader, tailscale-manager)
    - Goal: Eliminate circular registry self-management and simplify MemberRole state recovery

II. Medium term goals (investigatory)

[ ] Add etcd status endpoint, for easy and comprehensive diagnostics of etcd related issues

[ ] Implement automated dead etcd member pruning in TS Manager (Recruiter)
    [ ] Detect offline/unresponsive nodes during TS Manager reconciliation cycles
    [ ] Evict unreachable members via etcd API prior to adding new nodes
        - Resolves etcd "unhealthy cluster" blocks on AddLearner when members are down

III. Immediate Goals (consider these first)

    [ ] 

IV. Idea bucket:

V. Bugs:
    [ ] Unhealthy cluster after vm node disabled
        - 3 nodes in an operational cluster:
            a. disable leader any of the 3 (qm stop xxx)
                -> other 2 race for leader, if required. this works
                -> they appear to continue or resume normal operations, as expected.
            b. add a fourth node to the cluster, excepting it to be assimilated
                -> the node appears to correctly start and is awaiting assimilation (by its logs)
                -> the ts manager of the cluster attempts to recruit the new node, but it just loops with etcd error, like below
                    {"level":"warn","ts":"2026-08-24T14:27:52.101759Z","logger":"etcd-client","caller":"v3@v3.5.23/retry_interceptor.go:63","msg":"retrying of unary invoker failed","target":"etcd-endpoints://0xc0002ab860/localhost:2379","attempt":0,"error":"rpc error: code = Unavailable desc = etcdserver: unhealthy cluster"}
                    2026/08/24 14:27:52 [TSMgr] host=kaffcloud-home-103 step=add_learner: add learner http://100.124.164.71:2380: etcdserver: unhealthy cluster
        I would have though a cluster with 2/3 nodes responding was healthy, and able to take on new members.
        