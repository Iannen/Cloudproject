I. Long term goals

II. Medium term goals
    - [x] Configuration and Path Centralization
        - Consolidate duplicate etcd key path helpers into config/config.go and remove redundant helper definitions from adapters/etcd-store.go
        - Move hardcoded key prefix strings (e.g., "heartbeats/nodes/", "assignments/definitions/") into config constants
        - Move operational timeouts, ticker intervals, and retry durations across roles/infra into central config constants
        - Centralize default host directory paths, etcd/HTTP ports, and node naming prefix filters ("kaffcloud")

- [ ] Eliminating EtcdSession & SessionWrapper in codebase
        - [ ] go-controller/src/adapters/etcd-store.go
            - Delete SessionWrapper interface, EtcdSession struct, and its methods (Done, Close, LeaseID).
            - Change return type of NewSession to (*concurrency.Session, error).
            - Update PutWithSession and TryClaimLeadership parameters from SessionWrapper to *concurrency.Session.
        - [ ] go-controller/src/roles/member.go
            - Update MemberStore interface methods (NewSession, PutWithSession, TryClaimLeadership) to accept/return *concurrency.Session.
            - Update MemberRole.Run method signature to accept sess *concurrency.Session.
        - [ ] go-controller/src/roles/registry.go
            - Update RoleRunner interface method signature Run to accept sess *concurrency.Session.
        - [ ] go-controller/src/roles/leader.go
            - Update LeaderRole.Run method signature to accept sess *concurrency.Session.
        - [ ] go-controller/src/roles/ts-manager.go
            - Update TailscaleManagerRole.Run method signature to accept sess *concurrency.Session.
        - [ ] go-controller/src/roles/runtime.go
            - Update AssignmentRuntime.Start method signature to accept sess *concurrency.Session.

- [ ] Fix prompt MD file for smoother exp.
        - sick of getting BL items not in copybox
        - sick of inline comments.
        - gotta enforce code hygiene

III. Immediate Goals


IV. Idea bucket:


V. Known Bugs:
      - Tailscale bootstrap race condition: join-etcd-cluster.sh may time out after 10s if Tailscale IP allocation is slow.

      - Unauthenticated etcd port exposure: docker-compose.yml binds etcd to 0.0.0.0:2379.


HISTORY STASH (insert below)

I: Initialized .clank directory 
    - ran clanker in repo directory

II: Decoupled Container Dependencies in Docker Compose
    - Updated docker-compose.yml so the controller no longer depends on etcd at boot.
    - Set etcd restart policy to "no" to ensure it remains dormant on initial node boot.

III: Refactored Host Bootstrap Script
    - Modified join-etcd-cluster.sh to spin up only the controller container during initial host boot, leaving etcd stopped.

IV: Deferred gRPC Client & Store Initialization
    - Removed top-level clientv3 / adapters.Store setup from main.go dependency injection.
    - Moved etcd lifecycle control, gRPC client connection, and store binding into dynamic HTTP runtime operations (/initialize).

V: Fixed CreateAssignment Overwrite Bug
    - Updated go-controller/src/adapters/etcd-store.go to perform a read-modify-write append on node assignment lists rather than overwriting them with a single-element array.

VI: Resolved lostcancel Context Leak Warnings
    - Added defer cancelSess() in go-controller/src/roles/member.go to guarantee context cleanup across all loop exit paths.

VII: Implemented Idempotent /initialize Handler
    - Added Docker container existence checks (docker compose ps -q etcd) inside handleInitialize to safely exit early with 200 OK if etcd is already running on the host.

VIII: Developed Tailscale Manager Role & Peer Discovery
    - Implemented Tailscale status parsing to discover dormant nodes on the tailnet.
    - Added learner registration logic to add discovered candidate nodes to etcd before invoking assimilation.

IX: Implemented /assimilate & /activate Endpoints and Handlers
    - Created two-phase /assimilate and /activate endpoints on HttpServer to prevent gRPC learner errors.
    - Configured handlers to update local .env, purge stale etcd volumes, launch etcd in learner mode, and start gRPC member roles post-promotion.