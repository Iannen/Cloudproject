
I. Long term goals

II. Medium term goals

III. Immediate goals
      - Explore the nature of the current repository. 
      - Is it new? Then bootstrap a new project
      - Does it have content? Then analyze with user

IV. Idea bucket:

src.bak -> gemini says old news
      "
      Analysis of src.bak
      What this folder represents:

      src.bak is the pre-refactored snapshot of the Go controller codebase.

      It includes two instruction files (z1.refactor.instructions.specific and z2.refactor.instructions.general) detailing how to refactor the legacy codebase to use etcd/client/v3/concurrency sessions and robust control loop patterns.

      Relationship to src:

      The active src directory from your first prompt was created by applying the refactor instructions in z1.refactor.instructions.specific to src.bak.

      Key difference: In src.bak, adapters/etcd-store.go used a custom, manual KeepAliveLease implementation, and roles/member.go managed leases manually. In src, this was replaced with native concurrency.Session handling, dynamic lease IDs, and clean session teardown.
      "

bak.src -> even older:
      "
      What bak.src Is
      Monolithic main.go Orchestration: Instead of roles/member.go handling the reconciliation loop and etcd watch stream (like in src and src.bak), bak.src ran the watch loop and reconciliation functions directly inside main.go.

      Basic Stubs: The CampaignForLeadership method in adapters/etcd-store.go was hardcoded (_ = "magicstring"), and roles/member.go was stripped down to a basic node heartbeat background loop.

      Granular Heartbeats: Every AssignmentRuntime spun up its own isolated background goroutine calling KeepAliveLease on etcd.
      "


V. Known Bugs

HISTORY STASH (insert below)

I: Initialized .clank directory 
    - ran clanker in repo directory
