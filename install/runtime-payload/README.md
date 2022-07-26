# Building peer-pods payload image to be used with the Operator

- Build kata runtime and peer-pods binary from their respective sources
- Clone this repo locally and switch to the directory
- Copy the new binaries to this directory
- Create the payload container image
```
export REGISTRY=<registry/user>
docker build -t ${REGISTRY}/peer-pods-payload -f Dockerfile .
```
- Push the payload to the `REGISTRY`
```
docker push ${REGISTRY}/peer-pods-payload
```
