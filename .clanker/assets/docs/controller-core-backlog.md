I. Long term goals

[ ] add a server endpoint to view logs, so i dont have to drop into container all day

[ ] Reuse the output instruction in debloat.md

II. Medium term goals

[ ] Abstract repetitive ticker/ctx loop from LeaderRole.Run and Recruiter.Run into a shared reconciler runner helper.

    [ ] Align LeaderRole and its internal helpers to match Recruiter logging patterns and error handling return types.

[ ] (for consideration)Refactor context propagation and cleanup across roles and registry
    [ ] Remove stored r.ctx struct field in Registry and pass ctx directly through Registry.Start(ctx, a)
    [ ] Clean up redundant inner defer cancel() inside AssignmentRuntime.Start goroutine
    [ ] Preserve necessary child contexts in AssignmentRuntime (role isolation) and MemberRole (etcd session cleanup)
    [ ] Replace context.Background() in Recruiter.recruit defer cleanup with a bounded timeout context

III. Immediate Goals

[ ] Clean up member role
    Intent: Eliminate etcd leak (`clientv3`, `concurrency`) from domain code into adapter boundaries.
    Scope: `core/models/models.go`, `core/roles/member.go`, `adapters/etcd_store.go` (zero changes to `registry.go`).
    [ ] Abstract `concurrency.Session` into a domain session handle interface exposing lifecycle signals.
    [ ] Define domain-level event models/channels for member notifications in `models.go`.
    [ ] Replace direct watcher/ticker goroutines in `member.go` with an adapter-managed `SubscribeEvents(ctx, nodeID)` method.
    [ ] Move watch multiplexing, revision tracking, tickers, and reconnect handling into `etcd_store.go`.

IV. Idea bucket:

V. Known Bugs:
