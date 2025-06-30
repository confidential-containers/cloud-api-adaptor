package userdata

import (
	"context"
	"os"

	. "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/paths"
	dmidecode "github.com/fenglyu/go-dmidecode"
	cpuid "github.com/klauspost/cpuid/v2"
)

func isAzureVM() bool {
	return cpuid.CPU.HypervisorVendorID == cpuid.MSVM
}

func isAWSVM(ctx context.Context) bool {
	t, err := dmidecode.NewDMITable()
	if err != nil {
		return false
	}

	provider := t.Query(dmidecode.KeywordSystemManufacturer)

	return provider == "Amazon EC2"
}

func isGCPVM(ctx context.Context) bool {
	if cpuid.CPU.HypervisorVendorID != cpuid.KVM {
		return false
	}
	_, err := imdsGet(ctx, GcpImdsUrl, false, []kvPair{{"Metadata-Flavor", "Google"}})
	return err == nil
}

func hasUserDataFile() bool {
	_, err := os.Stat(UserDataPath)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

func isAlibabaCloudVM() bool {
	t, err := dmidecode.NewDMITable()
	if err != nil {
		return false
	}

	provider := t.Query(dmidecode.KeywordSystemManufacturer)

	return provider == "Alibaba Cloud"
}
