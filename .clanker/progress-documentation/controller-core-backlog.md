I. Long term items:

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

[ ] CTX propagation and usage razzia/analysis
    [ ] Eliminate stored context.Context fields from structs (e.g., RPCHandler) in favor of passing contexts explicitly via method calls
    [ ] Audit background goroutine spawns to ensure they use proper application-scoped contexts rather than request-scoped HTTP contexts
    [ ] Review detached cleanup contexts (like defer calls) to ensure they utilize bounded timeouts rather than raw context.Background()

III. Ran-into-trouble items:

IV. Purported actionable items:

V. Idea bucket:

VI. Bugs:

VII. Purported wisdom, for evaluation:

