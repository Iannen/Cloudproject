I. Long term goals (ignore)

II. Medium term goals (investigatory)

[ ] Add HTTP endpoint to expose container logs via Docker CLI
    [ ] Add GetLogs method to DockerMgr interface and adapter
        - execute 'docker logs <container_id>' and capture stdout/stderr output
    [ ] Register GET /logs route on NodeRole
        - fetch container logs using DockerMgr and return response body

[ ] Refactor context propagation and lifecycle cleanup in core roles
    [ ] Pass request context down through NodeRole activation methods
        - update handleInit, handleAssimilate, handleActivate to pass ctx to activateMember and store methods
    [ ] Simplify AssignmentRuntime lifecycle management
        - replace manual DoneChan synchronization with sync.WaitGroup or context wait patterns
    [ ] Retain custom Session abstraction to preserve core domain boundary
        - maintain standard library decoupling per core-doctrine rules

III. Immediate Goals (consider these first)

[ ] Implement error channel handling in NodeRole.Run
    [ ] Capture error channel returned by n.cms.Start
    [ ] Add select case to log errors received from server channel

IV. Idea bucket:

V. Known Bugs:
