Controller

I: Decoupled Container Dependencies in Docker Compose
    - Updated docker-compose.yml so the controller no longer depends on etcd at boot.
    - Set etcd restart policy to "no" to ensure it remains dormant on initial node boot.

II: Refactored Host Bootstrap Script
    - Modified join-etcd-cluster.sh to spin up only the controller container during initial host boot, leaving etcd stopped.

III: Deferred gRPC Client & Store Initialization
    - Removed top-level clientv3 / adapters.Store setup from main.go dependency injection.
    - Moved etcd lifecycle control, gRPC client connection, and store binding into dynamic HTTP runtime operations (/initialize).

IV: Fixed CreateAssignment Overwrite Bug
    - Updated go-controller/src/adapters/etcd-store.go to perform a read-modify-write append on node assignment lists rather than overwriting them with a single-element array.

V: Resolved lostcancel Context Leak Warnings
    - Added defer cancelSess() in go-controller/src/roles/member.go to guarantee context cleanup across all loop exit paths.

VI: Implemented Idempotent /initialize Handler
    - Added Docker container existence checks (docker compose ps -q etcd) inside handleInitialize to safely exit early with 200 OK if etcd is already running on the host.

VII: Developed Tailscale Manager Role & Peer Discovery
    - Implemented Tailscale status parsing to discover dormant nodes on the tailnet.
    - Added learner registration logic to add discovered candidate nodes to etcd before invoking assimilation.

VIII: Implemented /assimilate & /activate Endpoints and Handlers
    - Created two-phase /assimilate and /activate endpoints on HttpServer to prevent gRPC learner errors.
    - Configured handlers to update local .env, purge stale etcd volumes, launch etcd in learner mode, and start gRPC member roles post-promotion.

IX: Centralized Configuration and Operational Parameters
    - Consolidated duplicate etcd key path helpers from adapters/etcd-store.go into config/config.go.
    - Moved hardcoded key prefixes ("heartbeats/nodes/", "assignments/definitions/") to config constants.
    - Centralized operational timeouts, ticker intervals, retry durations, ports (HTTP/etcd peer), host paths, and node prefix filters ("kaffcloud").

X: Deprecated and Removed Legacy Python Controller
    - Removed the old Python-based controller codebase following full functional supersession by the Go controller implementation.

XI: Replaced EtcdSession and SessionWrapper with concurrency.Session
    - Deleted custom SessionWrapper interface and EtcdSession struct wrapper.
    - Standardized store interfaces and role execution signatures across MemberRole, LeaderRole, TSMgr, and AssignmentRuntime to accept *concurrency.Session directly.

XII: Consolidated Data Models, Enforced Package Isolation, and Cleaned Up Interface Naming
    - Centralized Domain Entities, Cluster Config, DTOs/RPC, and External Integration models into `models/models.go`.
    - Updated `config.go` to consume `models.RoleSpec` and preserved unexported scope for internal runtime types.
    - Standardized interface and dependency naming across core role components (`RoleMgr`, `ParticipantStore`, `AssignmentStore`, `ClusterMgr`, `DockerMgr`, `FileMgr`, `HTTPServer`, `HealthChecker`, `TailscaleMgr`, `RpcClient`).

XIII: NodeRole.handleInit Idempotency Upgrade
    - Replaced docker-based `IsEtcdRunning` check with internal state query via `n.reg.IsActive("member-" + nodeID)`.
    - Added `IsActive` method to `RoleMgr` interface and `Registry`.
    - Cleaned up unused `IsEtcdRunning` interface methods.

XIV: MemberRole Edge-Triggered Reconciliation & Session Delegation
    - Refactored `MemberRole` session establishment to delegate retries and backoff handling to `ParticipantStore.NewSession`.
    - Transitioned `MemberRole` watching mechanism to an edge-triggered model where `watchLoop` acts strictly as a signal producer.
    - Consolidated state fetching and assignment diffing into `MemberRole.reconcile`, eliminating duplicate logic in `runSession`.

XV: MemberRole Etcd Decoupling & Event Model Refactoring
    - Abstracted etcd concurrency session handling behind a domain `models.Session` interface.
    - Defined `MemberEvent` types (`ASSIGNMENT_CHANGE`, `LEADER_DELETED`, `RECONCILE_TICK`) and domain channels in `models.go`.
    - Shifted watch multiplexing, revision tracking, tickers, and reconnect handling out of `member.go` into `SubscribeEvents` inside `ParticipantStore`.

XVI: NodeRole HTTP Transport Boundary Refactoring
    - Abstracted HTTP server behind `HTTPServer` interface and `DomainHandler` func types in `core/roles/node.go`.
    - Removed `net/http` dependencies, status codes, and HTTP-specific error writing logic from `NodeRole`.
    - Routing and response handling now execute via domain handlers (`/initialize`, `/assimilate`, `/activate`).

XVII: Registry Domain Elevation and Interface Decoupling
    - Relocated `registry.go` to `go-controller/src/core/registry/registry.go`.
    - Fully decoupled `Registry` from concrete adapter packages by depending solely on domain interfaces in `core/roles`.
    - Introduced composite `StoreAdapter` interface in `core/roles/interfaces.go` combining `AssignmentStore`, `ParticipantStore`, `ClusterMgr`, and store lifecycle `Connect`.
    - Updated `main.go` to wire concrete adapters into `registry.NewRegistry`, explicitly passing HTTP client capabilities as distinct `HealthChecker` and `RpcClient` parameters.