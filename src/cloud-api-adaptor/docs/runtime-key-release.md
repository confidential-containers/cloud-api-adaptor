# Secure Key Release at Runtime

To request a key at runtime a Pod can invoke a local Key Broker Client (KBC) implemented in [Attestation Agent](https://github.com/confidential-containers/guest-components/tree/main/attestation-agent) (AA). An instance of AA is available as gRPC endpoint in the Pod's network namespace on port `50001`. Depending on the KBC implementation this might trigger a remote attestation exchange with an external [Key Broker Service](https://github.com/confidential-containers/kbs) (KBS).


## Example

The following code is supposed to run in a Peer Pod. It downloads a grpcurl binary, the respective proto files for the gRPC service, and triggers a request to retrieve a key. The example below is using an external KBS via the `cc_kbc` KBC implementation.

```bash
wget https://github.com/fullstorydev/grpcurl/releases/download/v1.8.7/grpcurl_1.8.7_linux_x86_64.tar.gz -O grpcurl.tar.gz
tar -xvf grpcurl.tar.gz
wget https://raw.githubusercontent.com/confidential-containers/guest-components/main/attestation-agent/protos/getresource.proto -O aa.proto
./grpcurl -proto aa.proto -plaintext -d @ localhost:50001 getresource.GetResourceService.GetResource <<EOM
{
  "ResourcePath": "/my_repo/resource_type/123abc",
  "KbcName":"cc_kbc",
  "KbsUri": "https://some-kbs.com"
}
EOM
```
