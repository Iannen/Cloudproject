I. Idea bucket:
- Add etcd status endpoint, for easy and comprehensive diagnostics of etcd related issues

II. Items to refine & QC:

[ ] Standardize tick-bound cancellation contexts across reconciliation loops
    - Ensure each reconcile tick derives a cancellable child context (with timeouts or loop-tick cancellation) to prevent overlapping/hanging concurrent runs in: [ ] LeaderRole
    [ ] Recruiter
    [ ] MemberRole.

[ ] Implement automated dead etcd member pruning in Recruiter (affects adapters!)
    [ ] Align logs, symbols & whatever else to use Recruiter terminology instead of legacy tsmgr
    [ ] Refactor Recruiter reconciliation loop into a sequential pipeline:
        - Phase 1: Collect raw observational data from Tailscale and etcd (ports)
        - Phase 2: Process helpers sequentially:
            a. Detect and evict dead/unresponsive members to clear etcd "unhealthy cluster" blocks
            b. Refetch/update state and evaluate remaining capacity to recruit new online Tailscale peers

III. Purported actionable items:

IV. Slated for implementation:

[ ] Implement tick-bound cancellation and asynchronous execution for Recruiter reconciliation
    [ ] 'go-controller/src/core/roles/recruiter.go': Refactor Recruiter.Run to consume event-owned context and cancellation
        - Refactor event processing loop to use a type switch on ev.Type for EventReconcileTick
        - Add inFlight sync/atomic.Bool to Recruiter struct and use t.inFlight.CompareAndSwap(false, true) for the non-blocking execution guard
        - Call ev.Cancel() immediately on the dropped event branch if an execution is already in flight
        - Spawn t.reconcile in a background goroutine passing ev.Ctx and deferring ev.Cancel() and t.inFlight.Store(false)
        - Remove inline context timeout instantiation (45s hardcoded duration) from core logic
        - Update t.reconcile signature from 't.reconcile(ctx, nodeID)' to 't.reconcile(ctx)'
    [ ] 'go-controller/src/core/models/events.go: Type Event'
        - Generalize 'Event' struct to support context propagation and lifetime management across all event producers:
            - Add 'Ctx context.Context' and unexported 'cancelFunc context.CancelFunc' fields to 'Event' struct
            - Implement 'Cancel()' method on 'Event' to safely invoke 'cancelFunc' if non-nil
            - Add constructor 'NewTickEvent(ctx context.Context, cancel context.CancelFunc) Event' specifically for tick-bound events carrying cancellation scope
            - Add constructor 'NewEvent(typ EventType) Event' for simple/un-scoped events (e.g., assignment changes, leader deletion)
    [ ] 'go-controller/src/adapters/etcd-store.go'
            -scope: SubscribeRecruiterEvents(ctx context.Context) (<-chan models.Event, error), runTicker(ctx context.Context, ch chan<- models.Event)
        - notes on etcd-store.go changes:
            - When creating tick events (EventReconcileTick), the ticker goroutine must derive a tick-bound context derived from the parent context using context.WithTimeout(ctx, timeout). Timeout must be sourced from new member 'recTimeout' on StoreConfig, its valued supplied inline in main go, similar to other adapter configs.
            - The generated tickCtx and tickCancel must be attached to the models.Event{Type: models.EventReconcileTick, Ctx: tickCtx, Cancel: tickCancel} payload before sending down the event channel.
            -Because channel sends (s.notifyEvent or direct select) can drop ticks under channel saturation/backpressure, any dropped event MUST call tickCancel() inside the adapter to prevent context timer leaks.
            -If successfully sent down the channel, ownership of calling Cancel() transfers to the consumer (Recruiter.Run).

        
V. Recently implemented:

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
