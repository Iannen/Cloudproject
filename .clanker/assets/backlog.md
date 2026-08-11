
I. Long term goals

II. Medium term goals

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
