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

[ ] Look for commonalities of the various roles, potentially leading refactors that save characters/tokens and presents a cleaner, more solid architecture

[ ] Evaluate main.go with regards to its imports and workings outside of the DI. What is the idiom 

III. Immediate Goals (consider these first)

[x] Standardize Assignment entity semantics across core domain (value vs pointer)
    [x] Align NewLeaderAssignment (returns value) and NewMemberAssignment (returns pointer) to value semantics
    [x] Align TSPeer and TSStatus to value semantics
    [x] Relocate Session interface definition from models/domain.go to models/session.go to align with doctrine

[x] Refactor roles to accept Assignment by value at construction
    [x] Pass Assignment by value to NewRole constructors instead of passing pointers in Run()
    [x] Remove redundant nodeID parameters from internal role execution methods. 
    [x] Update RoleRunner and Registry interfaces to reflect value semantics

IV. Idea bucket:

V. Bugs:
