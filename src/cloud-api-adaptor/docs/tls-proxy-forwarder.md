# TLS communication between `cloud-api-adaptor` and `agent-protocol-forwarder`

`cloud-api-adaptor` running in a worker node communicates with `agent-protocol-forwarder` running in a peer pod VM using a mutual TLS connection. This mTLS connection is used to transfer TTRPC requests and responses of [agent protocol](https://github.com/kata-containers/kata-containers/blob/main/src/libs/protocols/protos/agent.proto) between `kata-shim` and `kata-agent`.

There are two options to configure mTLS connections.

* Automatic configuration - `cloud-api-adaptor` automatically generates necessary keys and certificates
* Manual configuration - Users need to prepare necessary keys and certificates, and correctly deploy them.

Automatic configuration is a quick convenient option, but has some consideration points. If you want to fully control deployment of certificates and private keys, you can adopt the manual configuration option. You can also completely disable TLS encryption for testing purpose.

## Automatic configuration

Automatic TLS configuration is enabled by default. When command line options for TLS manual configuration are NOT specified, TLS certificates and keys are automatically generated and deployed appropriately.

Mutual TLS configuration consists of server and client authentications.

When the `-ca-cert-file` option of `cloud-api-adaptor` is NOT specified, a Certificate Authority (CA) service is enabled. The CA service issues a server certificate when `cloud-api-adaptor` creates a new peer pod VM. The generated certificate and private key are passed to the new pod VM as cloud-init data in a CreateInstance API call of cloud provider.

When the `-cert-file` and `-cert-key` options of `cloud-api-adaptor` are NOT specified, `cloud-api-adaptor` generates a self-signed client certificate and its private key, and passes the client certificate to the new peer pod VM as cloud-init data in a CreateInstance API call of cloud provider.

### Security consideration points

Note that a server private key is passed to a peer pod VM as cloud-init data in an API call of cloud provider. This seems that there is a security risk here, but the security risk is considered small in practice. TLS session keys reside in memory of a worker node. This means that cloud administrators can access the session keys and possibly decrypt TLS traffics, unless the worker node is in a secure enclave. While cloud administrators can access TLS session keys, passing private keys via cloud API does not significantly increase security risks. One possible attack scenario is that a malicious cloud administrator injects a malformed private key, and the golang standard crypto library has a vulnerability when parsing such malformed key.

In fact, this automatic TLS configuration increases attack surfaces. If you need automation of TLS certificate management while you want to minimize attack surface,  another possible option is to construct your own system based on the Kubernetes [cert-manager](https://cert-manager.io/) with the manual TLS configuration.

## Manual configuration

To enable TLS manually, you'll need the following:

- Client certificate and key: This is for the proxy to use.
  The files must be named as `client.crt` and `client.key`.
- Server certificate and key: This is for the agent-protocol-forwarder running inside the Pod VM.
  The `SubjectAlternateName (SAN)` of the certificate should have the name `podvm-server`.
  The files must be named as `tls.crt` and `tls.key`.
- CA certificate (Optional): This is required if you are using self-signed certificates.
  The file must be named as `ca.crt`

The file name restrictions for certificates is due to the current podvm image generation and deployment.
This will be removed in a future release.

Please note that tls enablement described in this document is specific to the control
plane communication between `cloud-api-adaptor` that is running in the K8s worker
node and the `agent-protocol-forwarder` that is running in the Pod VM. More
specifically the communication between the proxy, that is run via
`cloud-api-adaptor` and the `agent-protocol-forwarder` that is running inside the
Pod VM will be using TLS.

### Add the server certificate to the Pod VM image

- Copy the server certificate, key and (optionally) the CA certificate to `~/cloud-api-adaptor/podvm/files/etc/certificates`.
You will need CA certificate if using self-signed certificates.

- Build the Pod VM image

### Enable TLS settings for cloud-api-adaptor

- Update the options under the `TLS certificates for CAA-to-peer-pod communication`
  comment in `install/charts/peerpods/values.yaml`
- Deploy PeerPods using Helm charts (see [install/README.md](../install/README.md))

### Generate self-signed certificates
This is only recommended for dev/test scenarios

- Generate root CA cert
```
openssl req -newkey rsa:2048 \
  -new -nodes -x509 \
  -days 30 \
  -out ca.crt \
  -keyout ca.key \
  -subj "/C=IN/ST=KA/L=BLR/O=Self/OU=Self/CN=podvm_root_ca"
```

- Generate server key and certificate
Ensure SAN contains `podvm-server`.
```
openssl genrsa -out tls.key 2048

openssl req -new -key tls.key -out tls.csr \
  -subj "/C=IN/ST=KA/L=BLR/O=Self/OU=Self/CN=podvm-server"

openssl x509  -req -in tls.csr \
  -extfile <(printf "subjectAltName=DNS:podvm-server") \
  -CA ca.crt -CAkey ca.key  \
  -days 30 -sha256 -CAcreateserial \
  -out tls.crt
```

- Generate client key and certificate
```
  openssl genrsa -out client.key 2048

  openssl req -new -key client.key -out client.csr \
    -subj "/C=IN/ST=KA/L=BLR/O=Self/OU=Self/CN=podvm_client"

  openssl x509  -req -in client.csr \
    -extfile <(printf "subjectAltName=DNS:podvm_client") \
    -CA ca.crt -CAkey ca.key -out client.crt -days 30 -sha256 -CAcreateserial
```

## No TLS encryption

You can completely disable TLS encryption of agent protocol communication between `cloud-api-adaptor` and `agent-protocol-forwarder` by specifying the `-disable-tls` option to both `cloud-api-adaptor` and `agent-protocol-forwarder`.
