package azure

import (
	"net/netip"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
)

func sn(name, prefix string) *armnetwork.Subnet {
	return &armnetwork.Subnet{
		Name:       to.Ptr(name),
		Properties: &armnetwork.SubnetPropertiesFormat{AddressPrefix: to.Ptr(prefix)},
	}
}

func TestPeerPodPrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		subnets []*armnetwork.Subnet
		want    string
	}{
		{"docs example", []*armnetwork.Subnet{sn("aks-subnet", "10.224.0.0/16")}, "10.225.0.0/16"},
		{"skips occupied", []*armnetwork.Subnet{sn("aks-subnet", "10.224.0.0/16"), sn("other", "10.225.0.0/16")}, "10.226.0.0/16"},
		{"smaller mask", []*armnetwork.Subnet{sn("aks-subnet", "10.224.0.0/24")}, "10.224.1.0/24"},
		{"idempotent reuse", []*armnetwork.Subnet{sn("aks-subnet", "10.224.0.0/16"), sn("peerpod", "10.225.0.0/16")}, "10.225.0.0/16"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := peerPodPrefix(tc.subnets)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != netip.MustParsePrefix(tc.want) {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPeerPodPrefixErrors(t *testing.T) {
	if _, err := peerPodPrefix(nil); err == nil {
		t.Error("expected error for empty subnet list")
	}
	if _, err := peerPodPrefix([]*armnetwork.Subnet{sn("v6", "fd00::/64")}); err == nil {
		t.Error("expected error for IPv6 prefix")
	}
}
