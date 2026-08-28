I. Idea bucket:
- Add etcd status endpoint, for easy and comprehensive diagnostics of etcd related issues

II. Items to refine & QC:

[ ] Analyse structural similarities of LeaderRole, Recruiter and MemberRole.

[ ] Implement automated dead etcd member pruning in Recruiter (affects adapters!)
    [ ] Align logs, symbols & whatever else to use Recruiter terminology instead of legacy tsmgr
    [ ] Refactor Recruiter reconciliation loop into a sequential pipeline:
        - Phase 1: Collect raw observational data from Tailscale and etcd (ports)
        - Phase 2: Process helpers sequentially:
            a. Detect and evict dead/unresponsive members to clear etcd "unhealthy cluster" blocks
            b. Refetch/update state and evaluate remaining capacity to recruit new online Tailscale peers

[ ] Standardize context naming and propagation hierarchy across core roles and registry
    - Rename and align context variables to reflect exact lifecycles (`appCtx`, `memberCtx`, `sessionCtx`, `roleCtx`, `tickCtx`)
    - Refactor context propagation in `main.go`, `rpcHandler.go`, `registry.go`, `member.go`, `leader.go`, and `recruiter.go`:
        - `appCtx`: Global application process context derived in `main.go`
        - `memberCtx`: Lifetime context of `MemberRole` loop spawned by `RPCHandler`
        - `sessionCtx` (`sCtx`): Session-scoped context bound to active etcd lease in `MemberRole.Run()`
        - `roleCtx` (`asgCtx`): Assignment-scoped context created by `Registry` for managed tasks (`LeaderRole`, `Recruiter`)
        - `tickCtx` (`opCtx`): Bounded execution context with timeout passed into transient reconciliation cycles
    - Develop this bl item further:
        - arrive at a comprehensive and purposeful ctx hierarchy for any and all ctx existing in the system
        - arrive at a comprehensive ctx lc management implementation for all ctx
        - arrive at a similarly comprehensive naming policy for ctx'es -> helping readers understand what each one does

III. Slated for implementation:

IV. Recently implemented:

V. Bugs:
