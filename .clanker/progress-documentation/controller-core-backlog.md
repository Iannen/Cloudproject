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

[ ] Phase out core imports in acc with doctrine

II. Medium term goals (investigatory)

[ ] Add etcd status endpoint, for easy and comprehensive diagnostics of etcd related issues

III. Immediate Goals (consider these first)

[ ] Normalize reconcile loop channel acquisition across reconciling roles
    [ ] Refactor Leader role to acquire tick events via adapter event stream
        - Define Leader domain event types in models (e.g., LeaderEvent)
        - Update LeaderStore interface to supply an event channel instead of instantiating time.Ticker inline
        - Update LeaderRole.Run to process events from the supplied channel
    [ ] Refactor Recruiter role to acquire tick events via adapter event stream
        - Define Recruiter domain event types in models (e.g., RecruiterEvent)
        - Update ClusterMgr interface to supply an event channel instead of instantiating time.Ticker inline
        - Update Recruiter.Run to process events from the supplied channel

IV. Idea bucket:

V. Bugs:
