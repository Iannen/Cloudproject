I. Long term goals (ignore)

[ ] add a server endpoint to view logs, so i dont have to drop into container all day

[ ] Reuse the output instruction in debloat.md in the prompts 

II. Medium term goals (investigatory)

[ ] Investigate context propagation and cleanup throughout the system

III. Immediate Goals (consider these first)

[ ] Registry love: Elevate Registry into a domain-level orchestrator in core while fully decoupling it from low-level infrastructure dependencies.
    `registry.go`
        [ ] Relocate to `core/registry/registry.go`
        [ ] Interface Abstraction: Replace concrete adapter references with domain interfaces (composite `StoreAdapter` for etcd, distinct `HealthChecker` and `RpcClient` parameters for HTTP client capabilities).
    `main.go`
        [ ] Align with new design: Instantiate adapters and pass them to `registry.NewRegistry` via core interfaces (passing `httpCli` explicitly as distinct `HealthChecker` and `RpcClient` parameters).
    `go-controller/src/core/roles/interfaces.go`
        [ ] Retain `RoleMgr` so roles interact with Registry without importing `core/registry`.
        [ ] Define composite domain interfaces (like `StoreAdapter`) combining embedded role interfaces (`AssignmentStore`, `ParticipantStore`, `ClusterMgr`) and store lifecycle methods (`Connect`).

IV. Idea bucket:

V. Known Bugs:
