I. Idea bucket:
- Add etcd status endpoint, for easy and comprehensive diagnostics of etcd related issues

II. Items to refine & QC:

[ ] Standardize tick-bound cancellation contexts across all reconciliation loops
    - Ensure each reconcile tick derives a cancellable child context (with timeouts or loop-tick cancellation) to prevent overlapping/hanging concurrent runs in LeaderRole, Recruiter, and MemberRole.

[ ] Implement automated dead etcd member pruning in Recruiter (affects adapters!)
    [ ] Align logs, symbols & whatever else to use Recruiter terminology instead of legacy tsmgr
    [ ] Refactor Recruiter reconciliation loop into a sequential pipeline:
        - Phase 1: Collect raw observational data from Tailscale and etcd (ports)
        - Phase 2: Process helpers sequentially:
            a. Detect and evict dead/unresponsive members to clear etcd "unhealthy cluster" blocks
            b. Refetch/update state and evaluate remaining capacity to recruit new online Tailscale peers

III. Purported actionable items:

IV. Slated for implementation:
        
V. Ran-into-trouble items:

[x] Standardize context naming conventions, propagation, and usage across core components
    [x] 'main.go': rename referenced to logical app ctx from 'ctx' to 'app_ctx' in acc with doctrine
    [x] 'go-controller/src/core/roles/node.go': Refactor RPCHandler methods to accept and propagate an 'req_ctx' explicitly
        - Rename parameters from generic or blank contexts to req_ctx in HandleInit, HandleAssimilate, HandleActivate, and HandleGetLogs
        - Ensure background goroutines spawned in activateMember correctly receive 'app_ctx' instead of request-scoped bindings
    [x] 'go-controller/src/core/registry/registry.go': 
        - Ensure that the only context management that occurs in the registry is that concerned with passing its app_ctx into role Run() calls
        - Enforce app_ctx naming convention where it is currently not used. The ctx must be the actual app ctx for it to use that name
    [x] 'go-controller/src/adapters/etcd-store.go' 
        - refactor 'RemoveMember' to manage a ctx internally for purpose of etcd interaction(as required by etcd), removing it from args list of method
        - align relevant core itf
    [x] 'go-controller/src/core/roles/recruiter.go': 
        - align call to 'str.RemoveMember' and adjacent code with refactored 'RemoveMember' method

VI. Bugs:
