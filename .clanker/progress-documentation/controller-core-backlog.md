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

[ ] Move config and models to top-level packages to decouple core from adapters:
    go-controller/src/
    ├── config/       <-- Shared config & adapter config structs
    ├── models/       <-- Shared domain entities & DTOs
    ├── core/         <-- Pure business logic & interfaces (Consumer)
    ├── adapters/     <-- Infrastructure implementations (Supplier)
    └── main.go       <-- Wiring & initialization (Organizer)

[ ] Should the roles store the assignmentdefs they receive in Run on themselves, for ease of reference?

III. Immediate Goals (consider these first)

[ ] Standardize Assignment entity semantics across core domain (value vs pointer)
    [ ] Align NewLeaderAssignment (returns value) and NewMemberAssignment (returns pointer) to value semantics
    [ ] Align TSPeer and TSStatus to value semantics
    [ ] Relocate Session interface definition from models/domain.go to models/session.go to align with doctrine

IV. Idea bucket:

V. Bugs:
