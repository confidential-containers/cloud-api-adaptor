module github.com/confidential-containers/cloud-api-adaptor

go 1.20

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.9.0
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.4.0
	github.com/IBM-Cloud/bluemix-go v0.0.0-20230120122421-afb48116b8f1
	github.com/IBM-Cloud/power-go-client v1.5.3 // indirect
	github.com/IBM/go-sdk-core/v5 v5.15.0
	github.com/IBM/platform-services-go-sdk v0.54.0 // indirect
	github.com/IBM/vpc-go-sdk v0.43.0
	github.com/aws/aws-sdk-go-v2 v1.23.0
	github.com/aws/aws-sdk-go-v2/config v1.25.1
	github.com/aws/aws-sdk-go-v2/credentials v1.16.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.134.0
	github.com/containerd/containerd v1.6.8
	github.com/containerd/ttrpc v1.1.0
	github.com/containernetworking/plugins v1.1.1
	github.com/containers/podman/v4 v4.2.0
	github.com/coreos/go-iptables v0.6.0
	github.com/gogo/protobuf v1.3.2
	github.com/google/uuid v1.3.1
	github.com/opencontainers/runtime-spec v1.1.0-rc.1
	github.com/stretchr/testify v1.8.4
	github.com/vishvananda/netlink v1.2.1-beta.2
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f
	github.com/vmware/govmomi v0.33.1 // indirect
	golang.org/x/sys v0.15.0
	google.golang.org/grpc v1.56.3
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/cri-api v0.23.1
	libvirt.org/go/libvirt v1.9008.0
	libvirt.org/go/libvirtxml v1.9007.0
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4 v4.2.1
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4 v4.0.0
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi v1.1.0
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2 v2.2.1
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources v1.1.1
	github.com/Azure/go-autorest/autorest v0.11.27
	github.com/BurntSushi/toml v1.2.0
	github.com/IBM/ibm-cos-sdk-go v1.9.4
	github.com/avast/retry-go/v4 v4.5.1
	github.com/aws/aws-sdk-go-v2/service/eks v1.29.5
	github.com/aws/aws-sdk-go-v2/service/iam v1.22.5
	github.com/aws/aws-sdk-go-v2/service/s3 v1.38.5
	github.com/confidential-containers/cloud-api-adaptor/cloud-providers v0.8.0
	github.com/confidential-containers/cloud-api-adaptor/peerpod-ctrl v0.8.0
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f
	github.com/kata-containers/kata-containers/src/runtime v0.0.0-20231130163424-59d733fafdf6
	github.com/moby/sys/mountinfo v0.6.2
	github.com/pelletier/go-toml/v2 v2.1.0
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/cobra v1.7.0
	github.com/tj/assert v0.0.3
	golang.org/x/exp v0.0.0-20230224173230-c95f2b4c22f2
	k8s.io/api v0.26.0
	k8s.io/apimachinery v0.26.0
	k8s.io/client-go v0.26.0
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b
	sigs.k8s.io/e2e-framework v0.1.0
	sigs.k8s.io/kustomize v2.0.3+incompatible
	sigs.k8s.io/kustomize/api v0.12.1
	sigs.k8s.io/kustomize/kyaml v0.13.9
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.5.0 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.22 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.1.1 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.4.13 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.14.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.2.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.5.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.7.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.1.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.10.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.1.36 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.10.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.15.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.17.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.19.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.25.2 // indirect
	github.com/aws/smithy-go v1.17.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-errors/errors v1.0.1 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/analysis v0.21.4 // indirect
	github.com/go-openapi/errors v0.20.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/loads v0.21.2 // indirect
	github.com/go-openapi/runtime v0.26.0 // indirect
	github.com/go-openapi/spec v0.20.8 // indirect
	github.com/go-openapi/strfmt v0.21.7 // indirect
	github.com/go-openapi/swag v0.22.4 // indirect
	github.com/go-openapi/validate v0.22.1 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.13.0 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.0.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.2 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kdomanski/iso9660 v0.4.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/leodido/go-urn v1.2.3 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	go.mongodb.org/mongo-driver v1.11.3 // indirect
	go.opentelemetry.io/otel v1.14.0 // indirect
	go.opentelemetry.io/otel/trace v1.14.0 // indirect
	go.starlark.net v0.0.0-20200306205701-8dd3e2ee1dd5 // indirect
	golang.org/x/crypto v0.16.0 // indirect
	golang.org/x/net v0.19.0 // indirect
	golang.org/x/oauth2 v0.10.0 // indirect
	golang.org/x/term v0.15.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/klog/v2 v2.110.1 // indirect
	k8s.io/kube-openapi v0.0.0-20221012153701-172d655c2280 // indirect
	sigs.k8s.io/controller-runtime v0.14.1 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

// The following line is a workaround for the issue descrined in https://github.com/containerd/ttrpc/issues/62
// We can remove this workaround when Kata stop using github.com/gogo/protobuf
replace google.golang.org/genproto => google.golang.org/genproto v0.0.0-20180817151627-c66870c02cf8

replace github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.14.0

replace github.com/confidential-containers/cloud-api-adaptor/cloud-providers => ./cloud-providers

replace github.com/confidential-containers/cloud-api-adaptor/peerpod-ctrl => ./peerpod-ctrl
