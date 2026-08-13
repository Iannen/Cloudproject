I. Long term goals

II. Medium term goals

- [ ] Develop Recruiter Role & Tailnet Discovery
      Once elected leader, assign the node the Recruiter role to discover dormant Tailnet peers and invoke /assimilate to add them as etcd learners.

- [ ] Implement /assimilate Endpoint
      Add handler to join incoming dormant nodes to the cluster as etcd learners and initialize their local roles.


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