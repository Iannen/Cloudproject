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
    [x] Refactor store.NewSession to handle backoffs & retries
        - Old signature: NewSession(ctx context.Context, ttl int64) (*concurrency.Session, error)
        - New signature: NewSession(ctx context.Context, ttl int64, retries int, interval time.Duration) (*concurrency.Session, error)
    [x] Delegate session retry loop in MemberRole to ParticipantStore using new configuration parameters
    [ ] Investigate moving the session acquistion out of the for loop, considering it a setup phase of the member. 

IV. Idea bucket:

V. Known Bugs:

