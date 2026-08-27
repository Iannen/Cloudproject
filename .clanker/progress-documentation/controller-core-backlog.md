I. Long term North Star items:

II. Items that need further refining & QC:

[ ] Standardize tick-bound cancellation contexts across all reconciliation loops
    - Ensure each reconcile tick derives a cancellable child context (with timeouts or loop-tick cancellation) to prevent overlapping/hanging concurrent runs in LeaderRole, Recruiter, and MemberRole.

III. Purported actionable items:/recruiter.go

[ ] CTX naming standardization, propagation and usage 
    [ ] Adopt explicit ctx naming conventions: app_ctx for application-scoped lifecycle contexts, and req_ctx for HTTP request-bound contexts
        -> go-controller/src/core/roles/node.go, go-controller/src/core/registry/registry.go 
    [ ] Audit background goroutine spawns in RPCHandler and Registry to ensure they use app_ctx rather than request-scoped contexts
        -> go-controller/src/core/roles/node.go, go-controller/src/core/registry/registry.go
    [ ] Pass req_ctx explicitly through handler method calls to ensure proper request cancellation, timeouts, and error code propagation (400/500)
        -> go-controller/src/core/roles/node.go
    [ ] Ensure cleanup operations rely on adapter-level configured timeouts rather than manual context manipulation in core. (affects adapters!)
        -> go-controller/src/core/roles/recruiter.go

[ ] Implement automated dead etcd member pruning in Recruiter (affects adapters!)
    [ ] Align logs, symbols & whatever else to use Recruiter terminology instead of legacy tsmgr
    [ ] Refactor Recruiter reconciliation loop into a sequential pipeline:
        - Phase 1: Collect raw observational data from Tailscale and etcd (ports)
        - Phase 2: Process helpers sequentially:
            a. Detect and evict dead/unresponsive members to clear etcd "unhealthy cluster" blocks
            b. Refetch/update state and evaluate remaining capacity to recruit new online Tailscale peers

IV. Ran-into-trouble items:

V. Idea bucket:
    [ ] Add etcd status endpoint, for easy and comprehensive diagnostics of etcd related issues

VI. Bugs:

VII. Purported wisdom, for evaluation:

