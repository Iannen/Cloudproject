I. Long term items:

[ ] Implement automated dead etcd member pruning in TS Manager (Recruiter)
    [ ] Detect offline/unresponsive nodes during TS Manager reconciliation cycles
    [ ] Evict unreachable members via etcd API prior to adding new nodes
        - Resolves etcd "unhealthy cluster" blocks on AddLearner when a node is down (e.g., stopping 1 node in a 3-node cluster causes `add_learner` to fail with `etcdserver: unhealthy cluster` when recruiting a 4th node)

II. Items under development:

[ ] Add etcd status endpoint, for easy and comprehensive diagnostics of etcd related issues

[ ] Look for commonalities of the various roles, potentially leading refactors that save characters/tokens and presents a cleaner, more solid architecture

[ ] Evaluate main.go with regards to its imports and workings outside of the DI. What is the idiom 
    
[ ] Refactor registry package to simplify runtime lifecycle management
    [ ] Eliminate AssignmentRuntime struct and inline goroutine management into Registry
    [ ] Track active assignments using a simple cancellation map (map[string]context.CancelFunc)
    [ ] Consolidate redundant teardown logic by removing StopManagedAssignments from Registry/RoleMgr in favor of StopAll
    [ ] Ensure Registry.StopAll gracefully waits on an internal sync.WaitGroup during teardown

[ ] Streamline models package types and event definitions (we must analyse any usages in the adapters package prior before this can proceed)
    [ ] Replace role-specific event types (MemberEvent, LeaderEvent, RecruiterEvent) with unified models.Event
        -> one has to see adapter logic first, i think?
    [ ] Remove unused models/handlers.go file and types
        -> if they are unused by core, but used by adapters, then they can be moved to adapters.
    [ ] Inline single-use RoleSpec struct into config.go
        -> yeah that i agree with. it just reduces indirection - right (new word for me)

[ ] Refactor main.go to adhere to idiomatic Go composition root practices
    [ ] Replace manual os.Signal channel handling with signal.NotifyContext
    [ ] Reorder shutdown sequence to ensure HTTP server stops prior to tearing down Registry runtimes

[ ] Improve error resilience and key formatting across roles
    [ ] Replace context.Background() in Recruiter defer cleanup with a bounded timeout context
    [ ] Centralize assignment and storage key formatting functions into core/models
        -> 
    [ ] Deduplicate channel closure and error-checking boilerplate across role event loops
        -> desire: unwrap errors at the adapter boundary so role loops only process valid events

[ ] CTX propagation and usage razzia/analysis
    [ ] Eliminate stored context.Context fields from structs (e.g., RPCHandler) in favor of passing contexts explicitly via method calls
        -> nay! registry and rpchandler will carry appctx on them, as they manage the LC of long lived routines
    [ ] Audit background goroutine spawns to ensure they use proper application-scoped contexts rather than request-scoped HTTP contexts
        -> we also must pass the http ctx into the method calls internal to the handlers, so that the request ctx can be cancelled if there is an issue(making the server return a 400 or 500 type response to whomever sent the req)
    [ ] Review detached cleanup contexts (like defer calls) to ensure they utilize bounded timeouts rather than raw context.Background()

III. Ran-into-trouble items:

IV. Purported actionable items:

[x] Clean up unused code and redundant struct dependencies across core packages
    [x] Remove unused dcr and osa struct fields from Registry in core/registry/registry.go
    [x] Move TSStatus struct definition from core/models/dto.go to the adapters package (it is used by tailscale.go)

V. Idea bucket:

VI. Bugs:

VII. Purported wisdom, for evaluation:

