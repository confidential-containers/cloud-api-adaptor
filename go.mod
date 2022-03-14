module github.com/confidential-containers/cloud-api-adaptor

go 1.16

require (
	github.com/IBM/go-sdk-core/v5 v5.6.3
	github.com/IBM/vpc-go-sdk v1.0.1
	github.com/aws/aws-sdk-go-v2 v1.15.0 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.15.0 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.10.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.31.0 // indirect
	github.com/containerd/containerd v1.5.7
	github.com/containerd/ttrpc v1.0.2
	github.com/containernetworking/plugins v0.9.1
	github.com/coreos/go-iptables v0.6.0
	github.com/gogo/protobuf v1.3.2
	github.com/google/uuid v1.3.0
	github.com/kata-containers/kata-containers/src/runtime v0.0.0-00010101000000-000000000000
	github.com/onsi/gomega v1.16.0 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/vishvananda/netlink v1.1.1-0.20210924202909-187053b97868
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f
	golang.org/x/sys v0.0.0-20211004093028-2c5d950f24ef
	google.golang.org/grpc v1.41.0 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/yaml.v2 v2.4.0
)

replace (
	github.com/kata-containers/kata-containers/src/runtime => ../kata-containers/src/runtime
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20180817151627-c66870c02cf8
)
