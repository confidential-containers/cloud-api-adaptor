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
