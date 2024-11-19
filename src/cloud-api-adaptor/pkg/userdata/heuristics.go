package userdata

import (
	"context"
	"os"

	"github.com/klauspost/cpuid/v2"
)

func isAzureVM() bool {
	return cpuid.CPU.HypervisorVendorID == cpuid.MSVM
}

func isAWSVM(ctx context.Context) bool {
	if cpuid.CPU.HypervisorVendorID != cpuid.KVM {
		return false
	}
	_, err := imdsGet(ctx, AWSImdsUrl, false, nil)
	return err == nil
}

func isGCPVM(ctx context.Context) bool {
	if cpuid.CPU.HypervisorVendorID != cpuid.KVM {
		return false
	}
	_, err := imdsGet(ctx, GcpImdsUrl, false, []kvPair{{"Metadata-Flavor", "Google"}})
	return err == nil
}

func isDockerContainer() bool {
	_, err := os.ReadFile("/.dockerenv")
	return err == nil
}
