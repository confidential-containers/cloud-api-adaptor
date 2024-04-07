package main

import (
	"fmt"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/kubemgr"
	test "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/securecomms"
)

func main() {
	err := kubemgr.InitKubeMgrInVitro()
	if err != nil {
		fmt.Printf("Failed to initialize KubeMgr: %v\n", err)
		return
	}
	test.PP()
}
