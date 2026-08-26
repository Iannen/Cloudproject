I. Long term items:

[ ] Redesign role lifecycle architecture into a 3-tier hierarchy
    - Tier 1 (NodeRole): Maintained directly by main.go as host process; exempt from registry
    - Tier 2 (MemberRole): Managed directly by NodeRole via RPC/HTTP activation; exempt from registry; manages etcd session and elections
    - Tier 3 (Dynamic Roles): Workloads managed exclusively by MemberRole via Registry (e.g., leader, tailscale-manager)
    - Goal: Eliminate circular registry self-management and simplify MemberRole state recovery

[ ] Implement automated dead etcd member pruning in TS Manager (Recruiter)
    [ ] Detect offline/unresponsive nodes during TS Manager reconciliation cycles
    [ ] Evict unreachable members via etcd API prior to adding new nodes
        - Resolves etcd "unhealthy cluster" blocks on AddLearner when a node is down (e.g., stopping 1 node in a 3-node cluster causes `add_learner` to fail with `etcdserver: unhealthy cluster` when recruiting a 4th node)

II. Items under development:

[ ] Add etcd status endpoint, for easy and comprehensive diagnostics of etcd related issues

[ ] Phase out core imports in acc with doctrine

[ ] Look for commonalities of the various roles, potentially leading refactors that save characters/tokens and presents a cleaner, more solid architecture

[ ] Evaluate main.go with regards to its imports and workings outside of the DI. What is the idiom 

[ ] models package audit
    [ ] models.go: are all members of the contained models used for something in codebase?
    [ ] dto.go: look for smells 
    [ ] events.go: look for opportunities to generalize.
    
[ ] registry package: consider splitting up into registry.go and runtime.go
    -> runtime is encapsulated by the registry, so contract outward to system should remain the same.
    -> then look for opportunities to consolidate runtime logic into the registry, to effect a consolidation.

III. Ran-into-trouble items:

IV. Purported actionable items:

[ ] Refactor node.go and decouple it from  the registry
    [ ] Remove all references to noderole from the registry
    [ ] Rename 'NodeRole' into something to do with 'RPCHandler' (im sure there is a better name here, but lets go with that for now)
    [ ] Remove the unused Run method from RPCHandler and have main.go instantiate and manage it directly
    [ ] The responsibilities of 'RPCHandler' become
        - Generally to receive and handle all rpc (the adapter serving as a transport to which RPCHandler is agnostic)
        - It should be aware if the host is 'active' or not, in the sense of beeing part of a cloud or not.
            - currently it has the ability become a cloud member by either starting a cloud ('initialize') or joining a cloud ('handleAssimilate' and 'handleActivate in tandem)

[ ] Decouple core models from serialization by implementing adapter-local DTOs
    [ ] Scrub JSON and serialization tags from all structs in core/models/
        - Remove json struct tags from Assignment, TSPeer, and other core models
    [ ] Create adapter-local DTOs and mappers for HTTP and Tailscale adapters
        - Define tagged DTO structs inside adapters package and write explicit mapping functions
    [ ] Update Store adapter to handle persistence mapping independently of core domain models
        - Ensure etcd serialization uses adapter-local representations or direct primitives

[x] Refactor etcd readiness check from HTTP client to Docker adapter
    [x] 'WaitEtcdReady' implementation from adapters.HttpClientAdapter to adapters.DockerAdapter
    [x] add 'WaitEtcdReady' to roles.DockerMgr interface
    [x] remove obsolete 'HealthChecker' interface
    [x] Update dependency injection wiring in main.go 
    [x] Align registry.go as required
    [x] Update call sites inside node.go to use the docker interface

V. Idea bucket:

VI. Bugs:

VII. Purported wisdom, for evaluation:

-Control Flow Direction Flaws in main.go: In hexagonal architecture, the driving side (primary adapters like HTTP handlers) should invoke core/application use cases. Here, HTTP handlers are mapped directly to methods on roles.NodeRole, which bypasses a clean application service or use-case layer, muddying the entry point boundaries.

