module github.com/confidential-containers/cloud-api-adaptor

go 1.18

require (
	github.com/vmware/govmomi v0.29.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.1.2
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.1.0
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v3 v3.0.1
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork v1.1.0
	github.com/IBM/go-sdk-core/v5 v5.6.3
	github.com/IBM/vpc-go-sdk v1.0.1
	github.com/aws/aws-sdk-go-v2 v1.16.5
	github.com/aws/aws-sdk-go-v2/config v1.15.11
	github.com/aws/aws-sdk-go-v2/credentials v1.12.6
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.31.0
	github.com/containerd/containerd v1.6.6
	github.com/containerd/ttrpc v1.1.0
	github.com/containernetworking/plugins v1.1.1
	github.com/containers/podman/v4 v4.2.0
	github.com/coreos/go-iptables v0.6.0
	github.com/gogo/protobuf v1.3.2
	github.com/google/uuid v1.3.0
	github.com/opencontainers/runtime-spec v1.0.3-0.20211214071223-8958f93039ab
	github.com/stretchr/testify v1.8.0
	github.com/vishvananda/netlink v1.1.1-0.20220115184804-dd687eb2f2d4
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f
	golang.org/x/sys v0.0.0-20220722155257-8c9f86f7a55f
	google.golang.org/grpc v1.47.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/cri-api v0.23.1
	libvirt.org/go/libvirt v1.8002.0
	libvirt.org/go/libvirtxml v1.8002.0
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.0.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v0.5.1 // indirect
	github.com/asaskevich/govalidator v0.0.0-20200907205600-7a23bdc65eef // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.11.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.16.7 // indirect
	github.com/aws/smithy-go v1.11.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-openapi/errors v0.20.2 // indirect
	github.com/go-openapi/strfmt v0.21.1 // indirect
	github.com/go-playground/locales v0.13.0 // indirect
	github.com/go-playground/universal-translator v0.17.0 // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/golang-jwt/jwt v3.2.1+incompatible // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kata-containers/kata-containers/src/runtime v0.0.0-20220913141151-9b49a6ddc6fd // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/leodido/go-urn v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	go.mongodb.org/mongo-driver v1.7.5 // indirect
	golang.org/x/crypto v0.0.0-20220511200225-c6db032c6c88 // indirect
	golang.org/x/net v0.0.0-20220722155237-a158d28d115b // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20220624142145-8cd45d7dbd1f // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/go-playground/validator.v9 v9.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// The following line is a workaround for the issue descrined in https://github.com/containerd/ttrpc/issues/62
// We can remove this workaround when Kata stop using github.com/gogo/protobuf
replace google.golang.org/genproto => google.golang.org/genproto v0.0.0-20180817151627-c66870c02cf8
