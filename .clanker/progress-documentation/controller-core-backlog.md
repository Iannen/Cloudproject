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

[x] Normalize reconcile loop channel acquisition across reconciling roles
    [x] Refactor Leader role to acquire tick events via adapter event stream
        - Define Leader domain event types in models (e.g., LeaderEvent)
        - Update LeaderStore interface to supply an event channel instead of instantiating time.Ticker inline
        - Update LeaderRole.Run to process events from the supplied channel
    [x] Refactor Recruiter role to acquire tick events via adapter event stream
        - Define Recruiter domain event types in models (e.g., RecruiterEvent)
        - Update ClusterMgr interface to supply an event channel instead of instantiating time.Ticker inline
        - Update Recruiter.Run to process events from the supplied channel

[ ] Eliminate core 'time' imports in accordance with core doctrine
    [x] Eliminate 'time' usage in 'leader.go' by moving ID generation inside adapter boundary
    [x] Eliminate 'time' usage in 'member.go'
        - Update store.NewSession to handle retries internally using configured interval
        - Update store.PutWithSession to handle retries internally and accept nodeID instead of hbKey
        - Pass NodeHeartbeatPath configuration to store adapter on construction
    [ ] Eliminate 'time' usage in 'node.go'
        -> healthchecker itf should not need to be passed the interval as an arg. the underlying adapter should receive it in constructor.
    [ ] Eliminate 'time' usage in 'recruiter.go'
        -> 'time.Sleep(2 * time.Second)' can be eliminated by letting the call to 'str.PromoteMember' handle the issue internally in a best-practices way
    (time is permitted in config.go as it deals with types only)

IV. Idea bucket:

V. Bugs:
