
I. Long term goals

II. Medium term goals

II. Medium-Term Goals

- Init Process Refactor
        Implement Passive / Dormant Node State

        Update node bootstrap logic to default to a dormant state on launch, maintaining basic health heartbeats without attempting role elections or workload execution.

        Add Global Cluster State & /init-cloud Endpoint

        Replace /make-leader with an /init-cloud HTTP endpoint on the API server.

        Update /init-cloud to write a global cluster/state = "initialized" key into etcd to trigger cluster initialization.

        Implement etcd Election Campaign for Leader Race

        Transition member loops from dormant to active upon detecting cluster/state = "initialized".

        Integrate go.etcd.io/etcd/client/v3/concurrency election routines so active members race for leadership automatically during bootstrap and failover.

        Develop Recruiter Role & Tailnet Node Discovery

        Create a new recruiter role function registered in the role registry.

        Logic: Once a node wins leadership, assign itself the recruiter role to discover dormant nodes on the Tailnet and recruit them into the active cluster assignment pool.

III. Immediate goals
      - Explore the nature of the current repository. 
      - Is it new? Then bootstrap a new project
      - Does it have content? Then analyze with user

IV. Idea bucket:

V. Known Bugs:
      - CreateAssignment overwrites assignment array: go-controller/src/adapters/etcd-store.go replaces the node's assignment slice with a single-item array instead of performing a read-modify-write append.

      - Tailscale bootstrap race condition: join-etcd-cluster.sh may time out after 10s if Tailscale IP allocation is slow.

      - Unauthenticated etcd port exposure: docker-compose.yml binds etcd to 0.0.0.0:2379.

HISTORY STASH (insert below)

I: Initialized .clank directory 
    - ran clanker in repo directory
