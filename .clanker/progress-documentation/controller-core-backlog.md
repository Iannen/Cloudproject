I. Long term North Star items:

[ ] Implement automated dead etcd member pruning in TS Manager (Recruiter)
    [ ] Detect offline/unresponsive nodes during TS Manager reconciliation cycles
    [ ] Evict unreachable members via etcd API prior to adding new nodes
        - Resolves etcd "unhealthy cluster" blocks on AddLearner when a node is down (e.g., stopping 1 node in a 3-node cluster causes `add_learner` to fail with `etcdserver: unhealthy cluster` when recruiting a 4th node)

    [ ] Add etcd status endpoint, for easy and comprehensive diagnostics of etcd related issues

II. Items that need further refining & QC:

[ ] Look for commonalities of the various roles, potentially leading refactors that save characters/tokens and presents a cleaner, more solid architecture
       
III. Purported actionable items:

[ ] Refactor main.go to adhere to idiomatic Go composition root practices
    [ ] Replace manual os.Signal channel handling with signal.NotifyContext
    [ ] Reorder shutdown sequence to ensure HTTP server stops prior to tearing down Registry runtimes


[ ] CTX naming standardization, propagation and usage
    [ ] Adopt explicit ctx naming conventions: app_ctx for application-scoped lifecycle contexts, and req_ctx for HTTP request-bound contexts
    [ ] Audit background goroutine spawns in RPCHandler and Registry to ensure they use app_ctx rather than request-scoped contexts
    [ ] Pass req_ctx explicitly through handler method calls to ensure proper request cancellation, timeouts, and error code propagation (400/500)
    [ ] Review detached cleanup contexts (like defer calls) to utilize bounded timeouts rather than raw context.Background()

IV. Ran-into-trouble items:

V. Idea bucket:

VI. Bugs:

VII. Purported wisdom, for evaluation:

