## Enable TLS communication between proxy and agent-protocol-forwarder

To enable TLS, you'll need the following:

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

- Update the options under the `TLS_SETTINGS` comment in `kustomization.yaml` under the `install/overlays/{provider}` directory
- Deploy the operator

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

openssl req -new -key tls.key -days 30 -out tls.csr \
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

  openssl req -new -key client.key -days 30 -out client.csr \
    -subj "/C=IN/ST=KA/L=BLR/O=Self/OU=Self/CN=podvm_client"

  openssl x509  -req -in client.csr \
    -extfile <(printf "subjectAltName=DNS:podvm_client") \
    -CA ca.crt -CAkey ca.key -out client.crt -days 30 -sha256 -CAcreateserial
```
