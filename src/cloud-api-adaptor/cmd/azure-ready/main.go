// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

// azure-ready posts a "Ready" health goal-state to the Azure Fabric
// WireServer at 168.63.129.16. It is the minimum signal a guest must emit
// to convince Azure that provisioning succeeded.
//
// The flow follows the same three steps Afterburn implements:
//
//  1. GET  http://168.63.129.16/?comp=versions          (probe protocol versions)
//  2. GET  http://168.63.129.16/machine/?comp=goalstate (extract incarnation, container, instance)
//  3. POST http://168.63.129.16/machine?comp=health     (send <State>Ready</State>)
//
// No state is persisted, no files are written, and no inbound surface is
// exposed. The binary is intended to be run once at boot from a
// Type=oneshot systemd unit.
package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

const (
	wireServer        = "168.63.129.16"
	defaultVersion    = "2015-04-05"
	httpHeaderVersion = "x-ms-version"
	httpHeaderAgent   = "x-ms-agent-name"
	agentName         = "azure-ready"

	defaultTimeout = 30 * time.Second
	defaultRetries = 30
	retrySleep     = 5 * time.Second
)

type versions struct {
	XMLName   xml.Name `xml:"Versions"`
	Supported struct {
		Version []string `xml:"Version"`
	} `xml:"Supported"`
}

type goalState struct {
	XMLName     xml.Name `xml:"GoalState"`
	Incarnation string   `xml:"Incarnation"`
	Container   struct {
		ContainerID  string `xml:"ContainerId"`
		RoleInstance struct {
			InstanceID string `xml:"InstanceId"`
		} `xml:"RoleInstanceList>RoleInstance"`
	} `xml:"Container"`
}

func main() {
	var (
		endpoint = flag.String("endpoint", "http://"+wireServer, "WireServer base URL")
		retries  = flag.Int("retries", defaultRetries, "number of attempts before giving up")
		timeout  = flag.Duration("timeout", defaultTimeout, "per-request timeout")
		verbose  = flag.Bool("v", false, "verbose logging")
	)
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("azure-ready: ")

	client := &http.Client{
		Timeout: *timeout,
		Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: *timeout}).DialContext,
			TLSHandshakeTimeout:   *timeout,
			ResponseHeaderTimeout: *timeout,
			DisableKeepAlives:     true,
		},
	}

	ctx := context.Background()
	var lastErr error
	for attempt := 1; attempt <= *retries; attempt++ {
		if err := report(ctx, client, *endpoint, *verbose); err != nil {
			lastErr = err
			log.Printf("attempt %d/%d failed: %v", attempt, *retries, err)
			time.Sleep(retrySleep)
			continue
		}
		log.Printf("reported Ready to %s", *endpoint)
		return
	}
	log.Fatalf("giving up after %d attempts: %v", *retries, lastErr)
}

func report(ctx context.Context, c *http.Client, endpoint string, verbose bool) error {
	version, err := negotiateVersion(ctx, c, endpoint, verbose)
	if err != nil {
		return fmt.Errorf("version negotiation: %w", err)
	}
	if verbose {
		log.Printf("negotiated WireServer version %s", version)
	}

	gs, err := fetchGoalState(ctx, c, endpoint, version, verbose)
	if err != nil {
		return fmt.Errorf("goal state: %w", err)
	}
	if verbose {
		log.Printf("incarnation=%s containerId=%s instanceId=%s",
			gs.Incarnation, gs.Container.ContainerID, gs.Container.RoleInstance.InstanceID)
	}

	return postHealth(ctx, c, endpoint, version, gs, verbose)
}

func negotiateVersion(ctx context.Context, c *http.Client, endpoint string, verbose bool) (string, error) {
	body, err := doRequest(ctx, c, http.MethodGet, endpoint+"/?comp=versions", "", "", nil, verbose)
	if err != nil {
		return "", err
	}
	var v versions
	if err := xml.Unmarshal(body, &v); err != nil {
		return "", fmt.Errorf("parse versions: %w", err)
	}
	for _, candidate := range v.Supported.Version {
		if candidate == defaultVersion {
			return candidate, nil
		}
	}
	if len(v.Supported.Version) > 0 {
		return v.Supported.Version[0], nil
	}
	return "", errors.New("WireServer returned no supported versions")
}

func fetchGoalState(ctx context.Context, c *http.Client, endpoint, version string, verbose bool) (*goalState, error) {
	body, err := doRequest(ctx, c, http.MethodGet, endpoint+"/machine/?comp=goalstate", version, "", nil, verbose)
	if err != nil {
		return nil, err
	}
	var gs goalState
	if err := xml.Unmarshal(body, &gs); err != nil {
		return nil, fmt.Errorf("parse goal state: %w", err)
	}
	if gs.Incarnation == "" || gs.Container.ContainerID == "" || gs.Container.RoleInstance.InstanceID == "" {
		return nil, fmt.Errorf("incomplete goal state: incarnation=%q container=%q instance=%q",
			gs.Incarnation, gs.Container.ContainerID, gs.Container.RoleInstance.InstanceID)
	}
	return &gs, nil
}

const healthBodyTemplate = `<?xml version="1.0" encoding="utf-8"?>
<Health>
  <GoalStateIncarnation>%s</GoalStateIncarnation>
  <Container>
    <ContainerId>%s</ContainerId>
    <RoleInstanceList>
      <Role>
        <InstanceId>%s</InstanceId>
        <Health>
          <State>Ready</State>
        </Health>
      </Role>
    </RoleInstanceList>
  </Container>
</Health>`

func postHealth(ctx context.Context, c *http.Client, endpoint, version string, gs *goalState, verbose bool) error {
	body := fmt.Sprintf(healthBodyTemplate,
		xmlEscape(gs.Incarnation),
		xmlEscape(gs.Container.ContainerID),
		xmlEscape(gs.Container.RoleInstance.InstanceID),
	)
	_, err := doRequest(ctx, c, http.MethodPost, endpoint+"/machine?comp=health",
		version, "text/xml; charset=utf-8", []byte(body), verbose)
	return err
}

func doRequest(ctx context.Context, c *http.Client, method, url, version, contentType string, body []byte, verbose bool) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}
	if version != "" {
		req.Header.Set(httpHeaderVersion, version)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set(httpHeaderAgent, agentName)
	if verbose {
		log.Printf("%s %s", method, url)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, snippet(respBody))
	}
	return respBody, nil
}

func snippet(b []byte) string {
	const max = 256
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}
