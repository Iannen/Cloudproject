I. Idea bucket:
- Add etcd status endpoint, for easy and comprehensive diagnostics of etcd related issues

II. Items to refine & QC:

[ ] Standardize tick-bound cancellation contexts across reconciliation loops
    - Ensure each reconcile tick derives a cancellable child context (with timeouts or loop-tick cancellation) to prevent overlapping/hanging concurrent runs in: [ ] LeaderRole
    [x] Recruiter
    [ ] MemberRole.

[ ] Implement automated dead etcd member pruning in Recruiter (affects adapters!)
    [ ] Align logs, symbols & whatever else to use Recruiter terminology instead of legacy tsmgr
    [ ] Refactor Recruiter reconciliation loop into a sequential pipeline:
        - Phase 1: Collect raw observational data from Tailscale and etcd (ports)
        - Phase 2: Process helpers sequentially:
            a. Detect and evict dead/unresponsive members to clear etcd "unhealthy cluster" blocks
            b. Refetch/update state and evaluate remaining capacity to recruit new online Tailscale peers

III. Slated for implementation:

[x] Implement a Sealed Interface pattern and tick-bound cancellation for LeaderRole event stream
    [x] 'go-controller/src/core/models/events.go': Generalize tick event and add Leader marker interface
        - Rename 'RecruiterTickEvent' to 'TickEvent' carrying 'Ctx context.Context' and 'Cancel context.CancelFunc'
        - Define unexported marker interface 'LeaderEvent'
        - Update 'RecruiterEvent' and 'LeaderEvent' interfaces to both be satisfied by 'TickEvent'
        - Define additional 'LeaderEvent' concrete implementations as needed (e.g., 'LeaderChangeEvent')
    [x] 'go-controller/src/core/roles/interfaces.go' & 'go-controller/src/adapters/etcd-store.go': Update Leader event subscription
        - Update 'SubscribeLeaderEvents' store port and adapter signature to return '<-chan models.LeaderEvent'
        - Update adapter implementation to emit sealed Leader events
        - Derive tick-bound context using store config timeout and implement channel drop cancellation guards to prevent context leaks
    [x] 'go-controller/src/core/roles/leader.go': Refactor LeaderRole event loop and add handleEvent helper
        - Add 'inFlight atomic.Bool' field to 'LeaderRole' struct
        - Refactor 'LeaderRole.Run' loop to pass incoming 'models.LeaderEvent' directly to a new 't.handleEvent(ev)' helper method
        - Implement 'handleEvent(ev models.LeaderEvent)' using a Go type switch ('switch e := ev.(type)')
        - Use 'l.inFlight.CompareAndSwap(false, true)' guard on tick events, invoking 'e.Cancel()' on drop
        - Spawn 'l.reconcile' in a background goroutine passing 'e.Ctx', deferring 'e.Cancel()' and 'l.inFlight.Store(false)'
    [x] 'go-controller/src/core/roles/recruiter.go': Update Recruiter references to generalized TickEvent
        - Update 'Recruiter.handleEvent' type switch to match against 'models.TickEvent'

IV. Recently implemented:

[x] Implement tick-bound cancellation and asynchronous execution for Recruiter reconciliation
    [x] 'go-controller/src/core/roles/recruiter.go': Refactor Recruiter.Run to consume event-owned context and cancellation
        - Refactor event processing loop to use a type switch on ev.Type for EventReconcileTick
        - Add inFlight sync/atomic.Bool to Recruiter struct and use t.inFlight.CompareAndSwap(false, true) for the non-blocking execution guard
        - Call ev.Cancel() immediately on the dropped event branch if an execution is already in flight
        - Spawn t.reconcile in a background goroutine passing ev.Ctx and deferring ev.Cancel() and t.inFlight.Store(false)
        - Remove inline context timeout instantiation (45s hardcoded duration) from core logic
        - Update t.reconcile signature from 't.reconcile(ctx, nodeID)' to 't.reconcile(ctx)'
    [x] 'go-controller/src/core/models/events.go: Type Event'
        - Generalize 'Event' struct to support context propagation and lifetime management across all event producers:
            - Add 'Ctx context.Context' and unexported 'cancelFunc context.CancelFunc' fields to 'Event' struct
            - Implement 'Cancel()' method on 'Event' to safely invoke 'cancelFunc' if non-nil
            - Add constructor 'NewTickEvent(ctx context.Context, cancel context.CancelFunc) Event' specifically for tick-bound events carrying cancellation scope
            - Add constructor 'NewEvent(typ EventType) Event' for simple/un-scoped events (e.g., assignment changes, leader deletion)
    [x] 'go-controller/src/adapters/etcd-store.go'
            -scope: SubscribeRecruiterEvents(ctx context.Context) (<-chan models.Event, error), runTicker(ctx context.Context, ch chan<- models.Event)
        - notes on etcd-store.go changes:
            - When creating tick events (EventReconcileTick), the ticker goroutine must derive a tick-bound context derived from the parent context using context.WithTimeout(ctx, timeout). Timeout must be sourced from new member 'recTimeout' on StoreConfig, its valued supplied inline in main go, similar to other adapter configs.
            - The generated tickCtx and tickCancel must be attached to the models.Event{Type: models.EventReconcileTick, Ctx: tickCtx, Cancel: tickCancel} payload before sending down the event channel.
            -Because channel sends (s.notifyEvent or direct select) can drop ticks under channel saturation/backpressure, any dropped event MUST call tickCancel() inside the adapter to prevent context timer leaks.
            -If successfully sent down the channel, ownership of calling Cancel() transfers to the consumer (Recruiter.Run).
    [x] Implement a 'Sealed Interface' pattern for the Recruiter event stream
        Description:
        Replace the generic `models.Event` channel returned by `SubscribeRecruiterEvents` with a sealed `models.RecruiterEvent` interface to enforce compile-time type safety over the event set.

        IMPORTANT - Scope & Backwards Compatibility:
        Do NOT modify or remove the existing legacy `models.Event` struct or shared event constants. The Leader and Member reconcilers (and their store subscriptions) must remain completely untouched and fully operational on their current regime. Achieve this exclusively via additive changes—introducing brand-new types for the Recruiter alongside the legacy models.

        Acceptance Criteria:
        - [x] Add new, isolated types to `package models` without altering legacy `models.Event`:
        - [x] Define the unexported marker interface `RecruiterEvent`.
        - [x] Implement `RecruiterTickEvent` satisfying `RecruiterEvent`.
        - [x] Update only the `SubscribeRecruiterEvents` store port and adapter signature to return `<-chan models.RecruiterEvent` (leave Leader/Member subscription methods untouched).
        - [x] Align the adapter implementation logic to serve the refactored event handling.
        - [x] Refactor `Recruiter.handleEvent` to use a Go type switch (`switch e := ev.(type)`).

V. Bugs:
