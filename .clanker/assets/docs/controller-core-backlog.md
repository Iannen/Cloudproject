I. Long term goals

[ ] add a server endpoint to view logs, so i dont have to drop into container all day

[ ] Reuse the output instruction in debloat.md

[ ] Clean up member role
    [ ] ..

II. Medium term goals

[ ] NodeRole.handleInit idempotency upgrade
    [ ] Create new method on registry allowing a node to see if it is running a particular role (consider if similar functionality exists already)
    [ ] Make new method accessible to NodeRole via itf addition
    [ ] Replace indirect n.dcr.IsEtcdRunning Docker status check with internal n.reg.IsActive("member-" + nodeID) state query to determine initialization status.
    [ ] Remove orphaned IsEtcdRunning method where it may exist

III. Immediate Goals

IV. Idea bucket:

V. Known Bugs:



