package azure

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
)

// Pod VM cleanup on deprovisioning is only invoked if the provisioner satisfies
// this interface, and is silently skipped otherwise.
func TestAzureProvisionerImplementsPodVMInstanceHandler(t *testing.T) {
	var p pv.CloudProvisioner = &AzureCloudProvisioner{}
	if _, ok := p.(pv.PodVMInstanceHandler); !ok {
		t.Fatal("AzureCloudProvisioner does not satisfy pv.PodVMInstanceHandler, pod VM cleanup would be silently skipped")
	}
}

// Pod VMs are identified by their name prefix when cleaning them up. If the
// naming scheme changes, they would no longer be recognised and would leak.
func TestPodVMNamePrefixMatchesGeneratedNames(t *testing.T) {
	name := util.GenerateInstanceName("nginx-caa-5bddddbf56-7kfvf", "4ef7f83bbef5404d0db7915191b739ef535d97a17659a87851f3d33a4da3a936", 63)
	if !strings.HasPrefix(name, podVMNamePrefix) {
		t.Errorf("generated instance name %q does not start with %q", name, podVMNamePrefix)
	}
}

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
