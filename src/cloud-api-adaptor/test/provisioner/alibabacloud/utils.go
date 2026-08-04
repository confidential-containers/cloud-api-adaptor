// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package alibabacloud

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"time"

	ecs "github.com/alibabacloud-go/ecs-20140526/v4/client"
	"github.com/alibabacloud-go/tea/tea"
	vpc "github.com/alibabacloud-go/vpc-20160428/v6/client"
)

func credentialsFromEnv() (accessKeyID, accessKeySecret, securityToken string) {
	accessKeyID = os.Getenv("ALIBABACLOUD_ACCESS_KEY_ID")
	accessKeySecret = os.Getenv("ALIBABACLOUD_ACCESS_KEY_SECRET")
	securityToken = os.Getenv("ALIBABACLOUD_SECURITY_TOKEN")
	return
}

func ecsNameTag(name string) []*ecs.CreateSecurityGroupRequestTag {
	return []*ecs.CreateSecurityGroupRequestTag{
		{
			Key:   tea.String("Name"),
			Value: tea.String(name),
		},
	}
}

func vpcNameTags(name string) []*vpc.CreateVpcRequestTag {
	return []*vpc.CreateVpcRequestTag{
		{
			Key:   tea.String("Name"),
			Value: tea.String(name),
		},
	}
}

func vswitchNameTags(name string) []*vpc.CreateVSwitchRequestTag {
	return []*vpc.CreateVSwitchRequestTag{
		{
			Key:   tea.String("Name"),
			Value: tea.String(name),
		},
	}
}

func imageNameTags(name string) []*ecs.ImportImageRequestTag {
	return []*ecs.ImportImageRequestTag{
		{
			Key:   tea.String("Name"),
			Value: tea.String(name),
		},
	}
}

// parseIngressCIDRs parses a comma-separated list of IPv4 addresses or CIDRs.
// Bare IPs are normalized to /32.
func parseIngressCIDRs(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			part += "/32"
		}
		prefix, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, fmt.Errorf("invalid sg_ingress_cidrs entry %q: %w", part, err)
		}
		if !prefix.Addr().Is4() {
			return nil, fmt.Errorf("sg_ingress_cidrs only supports IPv4, got %q", part)
		}
		out = append(out, prefix.String())
	}
	return out, nil
}

// detectPublicIPv4 returns the host's current public IPv4 address.
// Used to scope security-group ingress to the GitHub Actions runner egress IP.
func detectPublicIPv4() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	urls := []string{
		"https://api.ipify.org",
		"https://checkip.amazonaws.com",
		"https://ifconfig.me/ip",
	}
	var errs []string
	for _, u := range urls {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		req.Header.Set("User-Agent", "cloud-api-adaptor-e2e")
		resp, err := client.Do(req)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
		resp.Body.Close()
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		if resp.StatusCode != http.StatusOK {
			errs = append(errs, fmt.Sprintf("%s: status %d", u, resp.StatusCode))
			continue
		}
		ip := strings.TrimSpace(string(body))
		if net.ParseIP(ip) == nil || net.ParseIP(ip).To4() == nil {
			errs = append(errs, fmt.Sprintf("%s: invalid IPv4 %q", u, ip))
			continue
		}
		return ip, nil
	}
	return "", fmt.Errorf("detect public IPv4: %s", strings.Join(errs, "; "))
}

func sanitizeOSSBucketName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	out := b.String()
	out = strings.Trim(out, "-")
	if len(out) > 63 {
		out = out[:63]
		out = strings.Trim(out, "-")
	}
	if len(out) < 3 {
		out = out + "-bucket"
	}
	return out
}
