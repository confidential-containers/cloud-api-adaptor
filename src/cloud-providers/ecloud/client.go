// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

// Package ecloud implements the cloud-api-adaptor provider for
// China Mobile Cloud (移动云, https://ecloud.10086.cn).
//
// The client talks to the central OpenAPI gateway at https://ecloud.10086.cn
// and signs requests with the AK/SK scheme documented at
// https://ecloud.10086.cn/op-help-center/doc/article/40688 (鉴权方式) and
// implemented by the official Go SDK (gitlab.ecloud.com/ecloud/ecloudsdkcore):
//
//	canonicalQueryString = sorted "percentEncode(k)=percentEncode(v)" pairs joined by "&"
//	stringToSign         = HTTPMethod + "\n" + percentEncode(path) + "\n" + sha256Hex(canonicalQueryString)
//	signature            = hex(HmacSHA256(stringToSign, "BC_SIGNATURE&" + secretKey))
package ecloud

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	defaultEndpoint    = "https://ecloud.10086.cn"
	signatureKeyPrefix = "BC_SIGNATURE&"
	signatureMethod    = "HmacSHA256"
	signatureVersion   = "V2.0"
	timestampFormat    = "2006-01-02T15:04:05Z"
	shanghaiTZ         = "Asia/Shanghai"
)

// API paths on the central gateway (gatewayPath from the API documentation).
const (
	pathCreateInstances        = "/api/openapi-instance/v4/create-instances"
	pathDeleteInstances        = "/api/openapi-instance/v4/delete-instances"
	pathDescribeInstance       = "/api/openapi-instance/v4/describe-instance"
	pathDescribeInstances      = "/api/openapi-instance/v4/list/describe-instances"
	pathDescribeInstanceStatus = "/api/openapi-instance/v4/list/describe-instance-status"
	pathDescribeFlavors        = "/api/openapi-instance/v4/region/flavors"
)

// API request/response models, following the ECS v4 OpenAPI documentation
// (https://ecloud.10086.cn/op-help-center/doc/article/96968).

type bootVolume struct {
	Size       int    `json:"size,omitempty"`
	VolumeType string `json:"volumeType,omitempty"`
}

type dataVolume struct {
	Size       int    `json:"size,omitempty"`
	VolumeType string `json:"volumeType,omitempty"`
}

type privateNetwork struct {
	NetworkID string `json:"networkId,omitempty"`
}

type publicNetwork struct {
	IPType        string `json:"ipType,omitempty"`
	BandwidthSize int    `json:"bandwidthSize,omitempty"`
}

type instanceTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type createInstancesRequest struct {
	ZoneID           string          `json:"zoneId,omitempty"`
	ImageID          string          `json:"imageId,omitempty"`
	FlavorName       string          `json:"flavorName,omitempty"`
	ChargeMode       string          `json:"chargeMode,omitempty"`
	Quantity         int             `json:"quantity,omitempty"`
	InstanceName     string          `json:"instanceName,omitempty"`
	KeyPairName      string          `json:"keyPairName,omitempty"`
	UserData         string          `json:"userData,omitempty"`
	BootVolume       *bootVolume     `json:"bootVolume,omitempty"`
	DataVolumes      []dataVolume    `json:"dataVolumes,omitempty"`
	PrivateNetwork   *privateNetwork `json:"privateNetwork,omitempty"`
	PublicNetwork    *publicNetwork  `json:"publicNetwork,omitempty"`
	SecurityGroupIDs []string        `json:"securityGroupIds,omitempty"`
	Tags             []instanceTag   `json:"tags,omitempty"`
}

type createInstancesResponseBody struct {
	ProcedureCode string   `json:"procedureCode"`
	OrderID       string   `json:"orderId"`
	InstanceIDs   []string `json:"instanceIds"`
}

type deleteInstancesRequest struct {
	InstanceIDs         []string `json:"instanceIds"`
	DeletePublicNetwork bool     `json:"deletePublicNetwork,omitempty"`
	DeleteDataVolumes   bool     `json:"deleteDataVolumes,omitempty"`
}

type instanceBatchResult struct {
	InstanceID string `json:"instanceId"`
	Result     bool   `json:"result"`
	Message    string `json:"message"`
}

type deleteInstancesResponseBody struct {
	InstanceBatchResult []instanceBatchResult `json:"instanceBatchResult"`
}

type instancePort struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	PrivateIPs []string `json:"privateIp"`
	MacAddress string   `json:"macAddress"`
	VpcName    string   `json:"vpcName"`
	SubnetName string   `json:"subnetName"`
}

type describeInstanceResponseBody struct {
	ID           string         `json:"id"`
	InstanceName string         `json:"instanceName"`
	Status       string         `json:"status"`
	FlavorName   string         `json:"flavorName"`
	ImageID      string         `json:"imageId"`
	ZoneID       string         `json:"zoneId"`
	RegionID     string         `json:"regionId"`
	CPU          int            `json:"cpu"`
	Memory       int            `json:"memory"`
	Disk         int            `json:"disk"`
	Ports        []instancePort `json:"ports"`
}

type instanceStatusEntry struct {
	InstanceID string `json:"instanceId"`
	Status     string `json:"status"`
}

type describeInstanceStatusResponseBody struct {
	Total   int                   `json:"total"`
	Content []instanceStatusEntry `json:"content"`
}

type fixedIPDetail struct {
	IPAddress  string `json:"ipAddress"`
	IPVersion  int    `json:"ipVersion"`
	FipAddress string `json:"publicIP"`
}

type portDetail struct {
	PrivateIP      string          `json:"privateIp"`
	FipAddress     string          `json:"fipAddress"`
	FipV6Address   string          `json:"fipV6Address"`
	PortID         string          `json:"portId"`
	FixedIPDetails []fixedIPDetail `json:"fixedIpDetailResps"`
}

type describeInstancesContent struct {
	InstanceID   string       `json:"instanceId"`
	InstanceName string       `json:"instanceName"`
	Status       string       `json:"status"`
	PortDetail   []portDetail `json:"portDetail"`
}

type describeInstancesRequest struct {
	InstanceID  string `json:"instanceId,omitempty"`
	ZoneID      string `json:"zoneId,omitempty"`
	WithNetwork bool   `json:"withNetwork,omitempty"`
	Page        int    `json:"page,omitempty"`
	PageSize    int    `json:"pageSize,omitempty"`
}

type describeInstancesResponseBody struct {
	Total   int                        `json:"total"`
	Content []describeInstancesContent `json:"content"`
}

type flavorInfo struct {
	VMType     string `json:"vmType"`
	CPU        int    `json:"cpu"`
	Ram        int    `json:"ram"` // memory size in GB
	FlavorName string `json:"flavorName"`
	FlavorType string `json:"flavorType"`
	SoldOut    string `json:"soldOut"`
}

type flavorsResponseBody struct {
	ZoneID      string       `json:"zoneId"`
	FlavorInfos []flavorInfo `json:"flavorInfos"`
}

// genericResponse is the envelope used by all APIs on the central gateway.
type genericResponse struct {
	RequestID    string          `json:"requestId"`
	State        string          `json:"state"`
	ErrorCode    string          `json:"errorCode"`
	ErrorMessage string          `json:"errorMessage"`
	Body         json.RawMessage `json:"body"`
}

// ecsClient is the mockable client interface used by the provider.
type ecsClient interface {
	CreateInstances(ctx context.Context, req *createInstancesRequest) (*createInstancesResponseBody, error)
	DeleteInstances(ctx context.Context, req *deleteInstancesRequest) (*deleteInstancesResponseBody, error)
	DescribeInstance(ctx context.Context, instanceID string, moreDetail bool) (*describeInstanceResponseBody, error)
	DescribeInstances(ctx context.Context, req *describeInstancesRequest) (*describeInstancesResponseBody, error)
	DescribeInstanceStatus(ctx context.Context, instanceIDs []string) (*describeInstanceStatusResponseBody, error)
	DescribeFlavors(ctx context.Context, zoneID string) ([]flavorsResponseBody, error)
}

type ecloudClient struct {
	endpoint   string
	accessKey  string
	secretKey  string
	regionID   string
	httpClient *http.Client
	// location used to render the Timestamp parameter. The gateway expects
	// the request time in the Asia/Shanghai timezone.
	location *time.Location
}

func newEcloudClient(endpoint, accessKey, secretKey, regionID string) (*ecloudClient, error) {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	loc, err := time.LoadLocation(shanghaiTZ)
	if err != nil {
		// Fall back to a fixed +08:00 zone if the tz database is unavailable
		loc = time.FixedZone("CST", 8*3600)
	}
	return &ecloudClient{
		endpoint:   strings.TrimSuffix(endpoint, "/"),
		accessKey:  accessKey,
		secretKey:  secretKey,
		regionID:   regionID,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		location:   loc,
	}, nil
}

// percentEncode implements the encoding rules of the signature spec: it is
// RFC3986 percent-encoding where space is %20 (not "+"), "*" is %2A and "~"
// is kept unescaped.
func percentEncode(s string) string {
	encoded := url.QueryEscape(s)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

// sign computes the signature query string (including the Signature parameter
// itself) for the given HTTP method, gateway path and API query parameters.
func (c *ecloudClient) sign(method, path string, queryParams map[string]string) string {
	params := make(map[string]string, len(queryParams)+5)
	for k, v := range queryParams {
		params[k] = v
	}
	params["AccessKey"] = c.accessKey
	params["Timestamp"] = time.Now().In(c.location).Format(timestampFormat)
	params["SignatureMethod"] = signatureMethod
	params["SignatureVersion"] = signatureVersion
	params["SignatureNonce"] = nonce()

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for i, k := range keys {
		builder.WriteString(percentEncode(k))
		builder.WriteString("=")
		builder.WriteString(percentEncode(params[k]))
		if i != len(keys)-1 {
			builder.WriteString("&")
		}
	}
	canonicalQueryString := builder.String()

	digest := sha256.Sum256([]byte(canonicalQueryString))
	stringToSign := strings.ToUpper(method) + "\n" + percentEncode(path) + "\n" + hex.EncodeToString(digest[:])

	mac := hmac.New(sha256.New, []byte(signatureKeyPrefix+c.secretKey))
	mac.Write([]byte(stringToSign))
	signature := hex.EncodeToString(mac.Sum(nil))

	return canonicalQueryString + "&Signature=" + percentEncode(signature)
}

// do executes a signed request against the central gateway and unmarshals the
// response body into out (which may be nil for empty bodies).
func (c *ecloudClient) do(ctx context.Context, method, path string, queryParams map[string]string, requestBody interface{}, out interface{}) error {
	signedQuery := c.sign(method, path, queryParams)
	reqURL := c.endpoint + path + "?" + signedQuery

	var bodyReader io.Reader
	if requestBody != nil {
		data, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.regionID != "" {
		req.Header.Set("Region-Id", c.regionID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s failed: %w", method, path, err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request %s %s returned HTTP %d: %s", method, path, resp.StatusCode, string(respData))
	}

	var envelope genericResponse
	if err := json.Unmarshal(respData, &envelope); err != nil {
		return fmt.Errorf("failed to unmarshal response envelope: %w (body: %s)", err, string(respData))
	}

	if !strings.EqualFold(envelope.State, "OK") {
		return fmt.Errorf("request %s %s returned error %s: %s", method, path, envelope.ErrorCode, envelope.ErrorMessage)
	}

	if out != nil {
		if len(envelope.Body) == 0 {
			return fmt.Errorf("request %s %s returned an empty body", method, path)
		}
		if err := json.Unmarshal(envelope.Body, out); err != nil {
			return fmt.Errorf("failed to unmarshal response body: %w (body: %s)", err, string(envelope.Body))
		}
	}

	return nil
}

func (c *ecloudClient) CreateInstances(ctx context.Context, req *createInstancesRequest) (*createInstancesResponseBody, error) {
	out := &createInstancesResponseBody{}
	if err := c.do(ctx, http.MethodPost, pathCreateInstances, nil, req, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ecloudClient) DeleteInstances(ctx context.Context, req *deleteInstancesRequest) (*deleteInstancesResponseBody, error) {
	out := &deleteInstancesResponseBody{}
	if err := c.do(ctx, http.MethodPost, pathDeleteInstances, nil, req, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ecloudClient) DescribeInstance(ctx context.Context, instanceID string, moreDetail bool) (*describeInstanceResponseBody, error) {
	query := map[string]string{"instanceId": instanceID}
	if moreDetail {
		query["moreDetail"] = "true"
	}
	out := &describeInstanceResponseBody{}
	if err := c.do(ctx, http.MethodGet, pathDescribeInstance, query, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ecloudClient) DescribeInstances(ctx context.Context, req *describeInstancesRequest) (*describeInstancesResponseBody, error) {
	out := &describeInstancesResponseBody{}
	if err := c.do(ctx, http.MethodPost, pathDescribeInstances, nil, req, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ecloudClient) DescribeInstanceStatus(ctx context.Context, instanceIDs []string) (*describeInstanceStatusResponseBody, error) {
	// Array query parameters are sent as repeated instanceIds entries. They
	// must be part of the canonical query string used for signing, so they
	// are flattened into indexed keys here and expanded again by do().
	query := map[string]string{}
	if len(instanceIDs) > 0 {
		query["instanceIds"] = strings.Join(instanceIDs, ",")
	}
	out := &describeInstanceStatusResponseBody{}
	if err := c.do(ctx, http.MethodGet, pathDescribeInstanceStatus, query, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ecloudClient) DescribeFlavors(ctx context.Context, zoneID string) ([]flavorsResponseBody, error) {
	query := map[string]string{}
	if zoneID != "" {
		query["zoneId"] = zoneID
	}
	var out []flavorsResponseBody
	if err := c.do(ctx, http.MethodGet, pathDescribeFlavors, query, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// nonce returns a random hexadecimal string used as SignatureNonce.
func nonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fall back to a timestamp-based nonce; collisions are harmless here
		// because the timestamp itself is already part of the signature.
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return hex.EncodeToString(b)
}
