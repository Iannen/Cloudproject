I. Long term goals

II. Medium term goals

[ ] Clean up member role
    [ ] ..
[ ] Clean up interface naming, placement, and member names
    [ ] `core/roles/interfaces.go`:
        - Rename `RegistryInterface` -> `RoleMgr` (keep in `interfaces.go`)
    [ ] `core/roles/node.go`:
        - Rename `DockerCreature` -> `DockerMgr`
        - Rename `OsCreature` -> `FileMgr`
        - Rename `ListenerCreature` -> `HTTPServer`
        - Rename `SpeakerCreature` -> `HealthChecker`
    [ ] `core/roles/leader.go`:
        - Rename `LeaderStore` -> `AssignmentStore`
    [ ] `core/roles/member.go`: ParticipantStore
        - Rename `MemberStore` -> `ParticipantStore`
    [ ] `core/roles/ts-manager.go`:
        - Rename `TsManagerStore` -> `ClusterMgr`
        - Rename `TailscaleProvider` -> `TailscaleMgr`
        - Rename `NodeClient` -> `RpcClient`

III. Immediate Goals

IV. Idea bucket:

V. Known Bugs:



