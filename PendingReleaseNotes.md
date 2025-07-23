# v3.15 Pending Release Notes

## Breaking changes

## Features

- helm: Support VolumeSnapshotClass and VolumeGroupSnapshotClass
- rbd: add support for [CSI Snapshot Metadata Service RPCs](https://github.com/container-storage-interface/spec/blob/master/spec.md#snapshot-metadata-service-rpcs)
- handle non graceful node shutdown [PR](https://github.com/ceph/ceph-csi/pull/5429/)
   - refer design doc for more details - [here](docs/design/proposals/non-graceful-node-shutdown.md)
- set nodeId:userId mapping in metadata [PR](https://github.com/ceph/ceph-csi/pull/5445)
   - refer design doc for more details - [here](https://github.com/ceph/ceph-csi/blob/devel/docs/design/proposals/userID-mapping.md)
- rbd: add support mix read/write qos limit for rbd volume

## NOTE

- `--setmetadata` flag has been set to true by default.
