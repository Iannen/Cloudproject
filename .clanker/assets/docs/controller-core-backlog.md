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

[ ] Refactor NodeRole HTTP transport boundary
    [ ] Replace `HTTPServer` interface in `core/roles/node.go` with domain-handler routing:
        ```go
        type DomainHandler func(ctx context.Context, body []byte) (string, error)

        type HTTPServer interface {
            RegisterGetRoute(pattern string, handler DomainHandler)
            RegisterPostRoute(pattern string, handler DomainHandler)
            Start(addr string, clientTimeout time.Duration)
            Shutdown(ctx context.Context) error
        }
        ```
    [ ] Remove `net/http` import and manual HTTP status/error-writing logic from `core/roles/node.go`
    [ ] Update `HTTPServerAdapter` to read request bodies, return HTTP 200 on success, and translate domain errors into standard HTTP status codes

III. Immediate Goals (consider these)

[ ] Clean up member role

IV. Idea bucket:

V. Known Bugs:
