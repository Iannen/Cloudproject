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

[ ] Categorize the models.go members appropriately into files (thinking models/domain.go models/dto.go)

[ ] Consider moving all interfaces used by the roles into roles/interfaces.go
    - while this is not idiomatic in go, it probably isnt a big violation and carries advantages in prompt engineering (not visible from inside the project)

[ ] Look for commonalities of the various roles, potentially leading refactors that save characters/tokens and presents a cleaner, more solid architecture

[ ] Evaluate wether the projects usage of pointers vs value is consistent & idiomatic (looking at you, NewLeaderAssignment and NewMemberAssignment)

[ ] Evaluate main.go with regards to its imports and workings outside of the DI. What is the idiom 

[ ] Move config and models to top-level packages to decouple core from adapters:
    go-controller/src/
    ├── config/       <-- Shared config & adapter config structs
    ├── models/       <-- Shared domain entities & DTOs
    ├── core/         <-- Pure business logic & interfaces (Consumer)
    ├── adapters/     <-- Infrastructure implementations (Supplier)
    └── main.go       <-- Wiring & initialization (Organizer)

III. Immediate Goals (consider these first)

[ ] Group adapter configuration into explicit structs within their respective adapter files and update constructors. Update main.go wiring per subtask completion
    The main file should not rely on config.go for the values, but supply them inline to the various config instantiations. the values no longer needed can be removed from config.go
    [x] Define DockerConfig in adapters package and refactor docker adapter (docker.go) constructor
    [x] Define StoreConfig in adapters package and refactor etcd store adapter (etcd-store.go) constructor
    [x] Define HTTPClientConfig in adapters package and refactor HTTP client adapter (http-client.go) constructor
    [x] Define HTTPServerConfig in adapters package and refactor HTTP server adapter (http-server.go) constructor
    [x] Define OsConfig in adapters package and refactor OS adapter (os.go) constructor
    [x] Define TailscaleConfig in adapters package and refactor Tailscale adapter (tailscale.go) constructor

[x] Further porting of config members to adapter layer.
    - main.go, when constructing 'StoreConfig' should not rely on config for 
        NodeAssignmentsPath: config.NodeAssignmentsPath,
		AsgDefPath:          config.AsgDefPath,'
    - instead the prefixes should be passed to the config constructor, and concatenation should be internal to the adapter (similar to how 'PrefixHeartbeats' and 'PrefixDefs' work)
    - item limited to those two points

[ ] Discuss with user how the NodeID propagation should be handled in the system. is it smelly to pass it down to the adapters from go, so that no core member need be concerned with it (we retain the option of gettning the nodeid from the adapter on-site)

IV. Idea bucket:

V. Bugs:
