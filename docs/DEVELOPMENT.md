## Prerequisites
- make
- golang 1.16+

## Clone source code
```
git clone -b CCv0-peerpod https://github.com/yoheiueda/kata-containers.git
git clone -b code-import https://github.com/yoheiueda/cloud-api-adaptor.git
```

## Build for AWS
```
cd cloud-api-adaptor
CLOUD_PROVIDER=aws make
```

## Build for IBMCloud
```
cd cloud-api-adaptor
CLOUD_PROVIDER=ibmcloud make
```

## Build for libvirt
Note that libvirt go library uses `cgo` and hence there is no static build.
Consequently you'll need to run the binary on the same OS/version where you have
built it.
You'll also need to install the libvirt dev packages before running the build.

```
cd cloud-api-adaptor
CLOUD_PROVIDER=libvirt make
```
