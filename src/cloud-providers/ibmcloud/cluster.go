// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"context"
	"fmt"
	"runtime"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/google/uuid"
)

type ClusterV2 struct {
	Service *core.BaseService
}

const DefaultServiceURL = "https://containers.cloud.ibm.com/global"

type ClusterOptions struct {
	Authenticator core.Authenticator
}

func NewClusterV2Service(options *ClusterOptions) (service *ClusterV2, err error) {
	serviceOptions := &core.ServiceOptions{
		URL:           DefaultServiceURL,
		Authenticator: options.Authenticator,
	}

	err = core.ValidateStruct(options, "options")
	if err != nil {
		err = core.SDKErrorf(err, "", "invalid-global-options", GetComponentInfo())
		return
	}

	baseService, err := core.NewBaseService(serviceOptions)
	if err != nil {
		err = core.SDKErrorf(err, "", "new-base-error", GetComponentInfo())
		return
	}

	service = &ClusterV2{
		Service: baseService,
	}

	return
}

func (clusterApi *ClusterV2) GetClusterTypeSecurityGroups(clusterID string) (result []securityGroup, response *core.DetailedResponse, err error) {
	return clusterApi.GetClusterTypeSecurityGroupsWithContext(context.Background(), clusterID)
}

func (clusterApi *ClusterV2) GetClusterTypeSecurityGroupsWithContext(ctx context.Context, clusterID string) (result []securityGroup, response *core.DetailedResponse, err error) {
	builder := core.NewRequestBuilder(core.GET)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = clusterApi.Service.GetEnableGzipCompression()

	// Construct the request URL
	_, err = builder.ResolveRequestURL(
		clusterApi.Service.Options.URL,
		"/network/v2/security-group/getSecurityGroups",
		nil,
	)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", GetComponentInfo())
		return
	}

	builder.AddQuery("cluster", clusterID)
	builder.AddQuery("type", "cluster")

	// Add headers
	sdkHeaders := GetHeaders("kubernetes_service_api", "V2", "GetSecurityGroups")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}

	builder.AddHeader("Accept", "application/json")

	// Build the request
	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", GetComponentInfo())
		return
	}

	var rawResponse []securityGroup
	response, err = clusterApi.Service.Request(request, &rawResponse)
	if err != nil {
		err = core.SDKErrorf(err, "", "http-request-err", GetComponentInfo())
		return
	}
	if rawResponse != nil {
		result = rawResponse
		response.Result = result
	}

	return
}

type securityGroup struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	UserProvided bool   `json:"userProvided"`
	Shared       bool   `json:"shared"`
	WorkerPoolID string `json:"workerPoolID"`
}

const (
	HEADER_NAME_USER_AGENT = "User-Agent"

	NAME = "cloud-api-adaptor-ibm"

	X_REQUEST_ID = "X-Request-Id"

	VERSION = "0.0.1"
)

func GetHeaders(serviceName string, serviceVersion string, operationId string) map[string]string {
	sdkHeaders := make(map[string]string)

	sdkHeaders[HEADER_NAME_USER_AGENT] = GetUserAgentInfo()
	sdkHeaders[X_REQUEST_ID] = GetNewXRequestID()

	return sdkHeaders
}

var UserAgent string = fmt.Sprintf("%s-%s %s", NAME, VERSION, GetSystemInfo())

func GetUserAgentInfo() string {
	return UserAgent
}

func GetNewXRequestID() string {
	return uuid.New().String()
}

var systemInfo = fmt.Sprintf("(arch=%s; os=%s; go.version=%s)", runtime.GOARCH, runtime.GOOS, runtime.Version())

func GetSystemInfo() string {
	return systemInfo
}

func GetComponentInfo() *core.ProblemComponent {
	return core.NewProblemComponent("github.com/confidential-containers/cloud-api-adaptor", VERSION)
}
