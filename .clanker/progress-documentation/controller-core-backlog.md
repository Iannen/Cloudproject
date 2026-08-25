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

[ ] Mitigate LLM Guardrail False Positives Across Projects
    - [ ] Sanitize Security-Adjacent Vocabulary: Map high-trigger operational terms to architectural equivalents across prompts:
        'bootstrap' / 'setup' → 'initialize' / 'instantiate'
        'hook' / 'intercept' → 'handler' / 'middleware'
        'kill' / 'destroy' → 'terminate' / 'dispose'
        'execute' / 'spawn' → 'run' / 'process'
        'inject' → 'pass dependency' / 'wire'
        'daemon' / 'agent' → 'background service'
    - [ ] Prepend Domain Context Headers: Frame prompt sessions explicitly as software refactoring or system design (e.g., "Context: Abstract software refactoring following Clean Architecture and Dependency Inversion patterns.").
    - [ ] Decouple Infrastructure from Core Domain: Keep low-level system operations (I/O, network sockets, OS calls, environment management) isolated behind interface boundaries/adapters so prompt payloads only expose pure business models and abstract contracts.

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

[ ] Should the roles store the assignmentdefs they receive in Run on themselves, for ease of reference?

III. Immediate Goals (consider these first)

IV. Idea bucket:

V. Bugs:
