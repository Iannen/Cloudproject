I. Long term goals (ignore)

[ ] Redesign role lifecycle architecture into a 3-tier hierarchy
    - Tier 1 (NodeRole): Maintained directly by main.go as host process; exempt from registry
    - Tier 2 (MemberRole): Managed directly by NodeRole via RPC/HTTP activation; exempt from registry; manages etcd session and elections
    - Tier 3 (Dynamic Roles): Workloads managed exclusively by MemberRole via Registry (e.g., leader, tailscale-manager)
    - Goal: Eliminate circular registry self-management and simplify MemberRole state recovery

[ ] Implement automated dead etcd member pruning in TS Manager (Recruiter)
    [ ] Detect offline/unresponsive nodes during TS Manager reconciliation cycles
    [ ] Evict unreachable members via etcd API prior to adding new nodes
        - Resolves etcd "unhealthy cluster" blocks on AddLearner when a node is down (e.g., stopping 1 node in a 3-node cluster causes `add_learner` to fail with `etcdserver: unhealthy cluster` when recruiting a 4th node)

II. Medium term goals (investigatory)

[ ] Add etcd status endpoint, for easy and comprehensive diagnostics of etcd related issues

[ ] Phase out core imports in acc with doctrine
    [ ] Analysis of 'time' usages in core:
        'config.go', basically fine
        'leader.go' -> push id generation inside adapter boundary, eliminate time from leader.go
        'member.go'
            -> eliminate first time usage by changing 'm.store.NewSession' to handle retries internally (having received the interval in constructor)
            -> eliminate second usage by similarly changing 'store.PutWithSession'. but we can also eliminate config.NodeHeartbeatPath usage by following the  pattern of passing the config value to the adapter on construction, and passing 'nodeId' to PutWithSession instead of 'hbKey'

III. Immediate Goals (consider these first)

IV. Idea bucket:

V. Bugs:
