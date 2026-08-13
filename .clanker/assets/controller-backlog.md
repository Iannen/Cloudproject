
I. Long term goals

II. Medium term goals

II. Medium-Term Goals

- [ ] Implement Passive/Dormant Node State
      Default node boot loop to dormant mode; run health heartbeats without starting workload schedulers or entering elections.

- [ ] Add Global Cluster State & /init-cloud Endpoint
      Replace /make-leader with /init-cloud to launch local etcd (if not running) and write cluster/state = "initialized" into KV store.

- [ ] Implement Election Campaigns for Leader Races
      Trigger dormant nodes to go active when cluster/state == "initialized". Use go.etcd.io/etcd/client/v3/concurrency routines for leader election and failover.

- [ ] Develop Recruiter Role & Tailnet Discovery
      Once elected leader, assign the node the Recruiter role to discover dormant Tailnet peers and invoke /assimilate to add them as etcd learners.


III. Immediate Goals

- [ ] 1. Decouple Container Dependencies in Compose
      Update docker-compose.yml so controller no longer depends on etcd, and set etcd restart policy to "no" so it stays dormant on boot.

- [ ] 2. Refactor Bootstrap Script
      Modify join-etcd-cluster.sh to only spin up the controller container during initial host boot.

- [ ] 3. Defer gRPC Client & Store Initialization
      Remove top-level clientv3 / adapters.Store setup from main.go DI. Move etcd lifecycle control and client instantiation into HTTP runtime operations (/init-cloud & /assimilate).


IV. Idea bucket:

V. Known Bugs:
      - CreateAssignment overwrites assignment array: go-controller/src/adapters/etcd-store.go replaces the node's assignment slice with a single-item array instead of performing a read-modify-write append.

      - Tailscale bootstrap race condition: join-etcd-cluster.sh may time out after 10s if Tailscale IP allocation is slow.

      - Unauthenticated etcd port exposure: docker-compose.yml binds etcd to 0.0.0.0:2379.

HISTORY STASH (insert below)

I: Initialized .clank directory 
    - ran clanker in repo directory
