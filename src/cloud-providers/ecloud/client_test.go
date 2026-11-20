// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ecloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPercentEncode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abcXYZ019-_.~", "abcXYZ019-_.~"},
		{"a b", "a%20b"},
		{"a+b", "a%2Bb"},
		{"a*b", "a%2Ab"},
		{"a/b", "a%2Fb"},
		{"2017-01-11T15:15:11Z", "2017-01-11T15%3A15%3A11Z"},
		{"中文", "%E4%B8%AD%E6%96%87"},
	}

	for _, tt := range tests {
		if got := percentEncode(tt.input); got != tt.expected {
			t.Errorf("percentEncode(%q) = %q, expect %q", tt.input, got, tt.expected)
		}
	}
}

func TestSign(t *testing.T) {
	client, err := newEcloudClient("https://ecloud.10086.cn", "testid", "testsecret", "cn-jiangsu-1")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	signed := client.sign("GET", "/api/openapi-instance/v4/describe-instance", map[string]string{
		"instanceId": "i-abc",
	})

	// The signed query string must be canonical: parameters sorted by name
	// and the Signature parameter appended last.
	params := strings.Split(signed, "&")
	if len(params) != 7 {
		t.Fatalf("expect 7 parameters, got %d: %s", len(params), signed)
	}

	keys := make([]string, len(params))
	values := map[string]string{}
	for i, kv := range params {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("invalid parameter %q", kv)
		}
		key, err := url.QueryUnescape(parts[0])
		if err != nil {
			t.Fatalf("failed to unescape key %q: %v", parts[0], err)
		}
		keys[i] = key
		values[key] = parts[1]
	}

	expectedOrder := []string{"AccessKey", "SignatureMethod", "SignatureNonce", "SignatureVersion", "Timestamp", "instanceId", "Signature"}
	for i, expected := range expectedOrder {
		if keys[i] != expected {
			t.Errorf("parameter %d: expect %q, got %q", i, expected, keys[i])
		}
	}

	if values["AccessKey"] != "testid" {
		t.Errorf("AccessKey = %q, expect %q", values["AccessKey"], "testid")
	}
	if values["SignatureMethod"] != signatureMethod {
		t.Errorf("SignatureMethod = %q, expect %q", values["SignatureMethod"], signatureMethod)
	}
	if values["SignatureVersion"] != signatureVersion {
		t.Errorf("SignatureVersion = %q, expect %q", values["SignatureVersion"], signatureVersion)
	}
	if len(values["SignatureNonce"]) != 32 {
		t.Errorf("SignatureNonce = %q, expect a 32-char hex string", values["SignatureNonce"])
	}
	// Signature must be a percent-encoded 64-char lowercase hex string (HmacSHA256)
	if got := values["Signature"]; len(got) != 64 {
		t.Errorf("Signature = %q, expect a 64-char hex string", got)
	}
}

func TestSignDeterministic(t *testing.T) {
	client, err := newEcloudClient("", "testid", "testsecret", "")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Parameters with identical names/values must produce identical canonical
	// ordering regardless of map iteration order. Only Timestamp,
	// SignatureNonce and Signature vary between invocations.
	query := map[string]string{"b": "2", "a": "1"}
	first := strings.Split(client.sign("POST", "/path", query), "&Signature=")[0]
	second := strings.Split(client.sign("POST", "/path", query), "&Signature=")[0]

	// Remove volatile parameters before comparison
	strip := func(s string) string {
		var kept []string
		for _, kv := range strings.Split(s, "&") {
			if strings.HasPrefix(kv, "Timestamp=") || strings.HasPrefix(kv, "SignatureNonce=") {
				continue
			}
			kept = append(kept, kv)
		}
		return strings.Join(kept, "&")
	}

	if strip(first) != strip(second) {
		t.Errorf("canonical query string is not deterministic: %q vs %q", strip(first), strip(second))
	}
}

// newTestServer creates an httptest server returning the given envelope body
// and recording the last request for assertions.
func newTestServer(t *testing.T, state string, body interface{}) (*httptest.Server, **http.Request, *string) {
	t.Helper()
	var lastReq *http.Request
	var lastBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastReq = r
		if r.Body != nil {
			data, _ := io.ReadAll(r.Body)
			lastBody = string(data)
		}

		envelope := map[string]interface{}{
			"requestId": "reqId-test",
			"state":     state,
		}
		if state == "OK" {
			raw, err := json.Marshal(body)
			if err != nil {
				t.Errorf("failed to marshal body: %v", err)
				return
			}
			envelope["body"] = json.RawMessage(raw)
		} else {
			envelope["errorCode"] = "SomeError"
			envelope["errorMessage"] = "some error message"
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(envelope)
	}))

	return server, &lastReq, &lastBody
}

func TestCreateInstancesRequest(t *testing.T) {
	server, lastReq, lastBody := newTestServer(t, "OK", createInstancesResponseBody{
		ProcedureCode: "POSTPAID",
		OrderID:       "MOP-O-1",
		InstanceIDs:   []string{"instance-id-1"},
	})
	defer server.Close()

	client, err := newEcloudClient(server.URL, "ak", "sk", "cn-jiangsu-1")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req := &createInstancesRequest{
		ZoneID:       "cn-jiangsu-1a",
		ImageID:      "image-1",
		FlavorName:   "s1.large.2",
		ChargeMode:   "HOUR",
		Quantity:     1,
		InstanceName: "podvm-test",
		UserData:     "dGVzdA==",
		PrivateNetwork: &privateNetwork{
			NetworkID: "network-1",
		},
		BootVolume: &bootVolume{
			Size:       40,
			VolumeType: "highPerformance",
		},
		SecurityGroupIDs: []string{"sg-1"},
		Tags:             []instanceTag{{Key: "k1", Value: "v1"}},
	}

	resp, err := client.CreateInstances(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateInstances failed: %v", err)
	}

	if len(resp.InstanceIDs) != 1 || resp.InstanceIDs[0] != "instance-id-1" {
		t.Errorf("unexpected instance IDs: %v", resp.InstanceIDs)
	}

	req2 := *lastReq
	if req2.Method != http.MethodPost {
		t.Errorf("method = %s, expect POST", req2.Method)
	}
	if req2.URL.Path != pathCreateInstances {
		t.Errorf("path = %s, expect %s", req2.URL.Path, pathCreateInstances)
	}
	if req2.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %s, expect application/json", req2.Header.Get("Content-Type"))
	}
	if req2.Header.Get("Region-Id") != "cn-jiangsu-1" {
		t.Errorf("Region-Id = %s, expect cn-jiangsu-1", req2.Header.Get("Region-Id"))
	}

	// The request URL must carry the signature parameters
	for _, key := range []string{"AccessKey", "Timestamp", "SignatureMethod", "SignatureVersion", "SignatureNonce", "Signature"} {
		if req2.URL.Query().Get(key) == "" {
			t.Errorf("missing signature parameter %s in request URL", key)
		}
	}

	// The request body must be a flat JSON object (no wrapper key)
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(*lastBody), &body); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if body["zoneId"] != "cn-jiangsu-1a" {
		t.Errorf("zoneId = %v, expect cn-jiangsu-1a", body["zoneId"])
	}
	if body["flavorName"] != "s1.large.2" {
		t.Errorf("flavorName = %v, expect s1.large.2", body["flavorName"])
	}
	pn, ok := body["privateNetwork"].(map[string]interface{})
	if !ok || pn["networkId"] != "network-1" {
		t.Errorf("privateNetwork = %v", body["privateNetwork"])
	}
}

func TestDescribeInstanceRequest(t *testing.T) {
	server, lastReq, _ := newTestServer(t, "OK", describeInstanceResponseBody{
		ID:     "instance-id-1",
		Status: "active",
		Ports: []instancePort{
			{ID: "port-1", PrivateIPs: []string{"192.168.1.10"}},
		},
	})
	defer server.Close()

	client, err := newEcloudClient(server.URL, "ak", "sk", "")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	resp, err := client.DescribeInstance(context.Background(), "instance-id-1", true)
	if err != nil {
		t.Fatalf("DescribeInstance failed: %v", err)
	}

	if resp.ID != "instance-id-1" || resp.Status != "active" {
		t.Errorf("unexpected response: %+v", resp)
	}

	req2 := *lastReq
	if req2.Method != http.MethodGet {
		t.Errorf("method = %s, expect GET", req2.Method)
	}
	if req2.URL.Query().Get("instanceId") != "instance-id-1" {
		t.Errorf("instanceId = %s", req2.URL.Query().Get("instanceId"))
	}
	if req2.URL.Query().Get("moreDetail") != "true" {
		t.Errorf("moreDetail = %s", req2.URL.Query().Get("moreDetail"))
	}
	// No Region-Id header should be set when region is empty
	if req2.Header.Get("Region-Id") != "" {
		t.Errorf("Region-Id = %s, expect empty", req2.Header.Get("Region-Id"))
	}
}

func TestDescribeFlavors(t *testing.T) {
	server, _, _ := newTestServer(t, "OK", []flavorsResponseBody{
		{
			ZoneID: "cn-jiangsu-1a",
			FlavorInfos: []flavorInfo{
				{VMType: "common", CPU: 2, Ram: 4, FlavorName: "s1.large.2", FlavorType: "s1", SoldOut: "0"},
			},
		},
	})
	defer server.Close()

	client, err := newEcloudClient(server.URL, "ak", "sk", "cn-jiangsu-1")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	flavors, err := client.DescribeFlavors(context.Background(), "cn-jiangsu-1a")
	if err != nil {
		t.Fatalf("DescribeFlavors failed: %v", err)
	}

	if len(flavors) != 1 || flavors[0].FlavorInfos[0].FlavorName != "s1.large.2" {
		t.Errorf("unexpected flavors: %+v", flavors)
	}
}

func TestErrorEnvelope(t *testing.T) {
	server, _, _ := newTestServer(t, "ERROR", nil)
	defer server.Close()

	client, err := newEcloudClient(server.URL, "ak", "sk", "")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.DeleteInstances(context.Background(), &deleteInstancesRequest{InstanceIDs: []string{"i-1"}})
	if err == nil {
		t.Fatal("expect an error for state=ERROR")
	}
	if !strings.Contains(err.Error(), "SomeError") || !strings.Contains(err.Error(), "some error message") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "forbidden")
	}))
	defer server.Close()

	client, err := newEcloudClient(server.URL, "ak", "sk", "")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.DescribeInstance(context.Background(), "i-1", false)
	if err == nil {
		t.Fatal("expect an error for HTTP 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("unexpected error: %v", err)
	}
}
