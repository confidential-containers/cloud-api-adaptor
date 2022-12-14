module github.com/confidential-containers/cloud-api-adaptor/volumes/csi-wrapper

go 1.18

require (
	github.com/container-storage-interface/spec v1.6.0
	google.golang.org/grpc v1.47.0
)

require (
	github.com/confidential-containers/cloud-api-adaptor v0.0.0-00010101000000-000000000000
	github.com/containerd/ttrpc v1.1.0
	github.com/emicklei/go-restful v2.15.0+incompatible // indirect
	github.com/gofrs/uuid v4.2.0+incompatible
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.5.2
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f
	k8s.io/apimachinery v0.24.1
	k8s.io/client-go v0.24.1
	k8s.io/code-generator v0.24.1
)

replace (
	github.com/confidential-containers/cloud-api-adaptor => ../../
	github.com/kata-containers/kata-containers/src/runtime => ../../../kata-containers/src/runtime
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20180817151627-c66870c02cf8
)
