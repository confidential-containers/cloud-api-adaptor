# VM information query service

`cloud-api-adaptor` provides a query service for VM ID information via an Unix domain socket (default: `/run/peerpod/hypervisor.sock`).

The query service is provided using [TTRPC](https://github.com/containerd/ttrpc), so you can implement a client to send a request via the Unix domain socket to query VM ID information using TTRPC.

The TTRPC protocol definition is as follows.

```
service PodVMInfo {
        rpc GetInfo(GetInfoRequest) returns (GetInfoResponse) {}
}

message GetInfoRequest {
    string PodName = 1;
    string PodNamespace = 2;

    bool Wait = 3;
}

message GetInfoResponse {
    string VMID = 1;
}
```

You need to specify the pod name and namespace name of a pod running in a peer pod VM in a `GetInfo` request. The query service responds with a VM ID.  The actual meaning of the VM ID value depends on the type of cloud provider. In the case of IBM Cloud, a VM ID is an ID of the virtual server instance (VSI).

When you need to update the protocol definition, edit [`proto/podvminfo/podvminfo.proto`](/proto/podvminfo/podvminfo.proto), and run [`hack/update-proto.sh`](/hack/update-proto.sh).
