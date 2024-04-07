package main

import (
	"fmt"
	"os"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/kubemgr"
	test "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/securecomms"
)

//var logger = log.New(log.Writer(), "[adaptor] ", log.LstdFlags|log.Lmsgprefix)

func main() {
	err := kubemgr.InitKubeMgrInVitro()
	if err != nil {
		fmt.Printf("Failed to initialize KubeMgr: %v\n", err)
		return
	}
	go test.PP()
	success := test.WN()

	if !success {
		os.Exit(-1)
	}
	fmt.Println("*** SUCCESS ***")
}
