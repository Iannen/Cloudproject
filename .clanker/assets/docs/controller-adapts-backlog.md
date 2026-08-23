I. Long term goals (ignore)

II. Medium term goals (investigatory)

[ ] the only domain import allowed is models. Other imports from domain are forbidden (such as config)
    [ ] Refactor `etcd-store.go` to remove imports of `core/config` by passing required paths and configuration values as parameters
    [x] Refactor `http-server.go` to remove import of `core/roles` by moving `DomainHandler` type definition to `models` package      

III. Immediate Goals (consider these first)

[ ] Adapters should not contain hardcoded values, nor should they access core/config directly.
    The values should be passed in the method, or supplied in the constructor.
    [ ] `go-controller/src/adapters/etcd-store.go`
        [ ] Remove import of `core/config`.
        [ ] Pass `leaderKey`, `reconcileInterval`, and `reconnectDelay` as parameters to `SubscribeEvents` (or via `Store` constructor) and forward them to watcher goroutines.
    [ ] `go-controller/src/adapters/http-client.go`
        [ ] Parameterize target base URL / port (`8080`) and path endpoints for `Assimilate` and `Activate`.
    [ ] `go-controller/src/adapters/docker.go`
        [ ] Move executable name (`docker`) and subcommand flags (`compose`, `up`, `down`, etc.) to configurable struct fields or method options.
    [x] `go-controller/src/adapters/os.go`
        [ ] Parameterize env file path (`.env`), file permissions (`0644`), and template format string in `WriteEnvConfig`.
    [x] `go-controller/src/adapters/tailscale.go`
        [ ] Parameterize executable name (`tailscale`) and env var key (`TAILSCALE_IP`).

IV. Idea bucket:

V. Known Bugs:



