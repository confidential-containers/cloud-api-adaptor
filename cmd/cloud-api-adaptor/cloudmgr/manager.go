// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cloudmgr

import (
	"flag"
	"os"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
)

type Cloud interface {
	ParseCmd(flags *flag.FlagSet)
	LoadEnv()
	NewProvider() (cloud.Provider, error)
}

var cloudTable map[string]Cloud = make(map[string]Cloud)

func Get(name string) Cloud {
	return cloudTable[name]
}

func defaultToEnv(field *string, env string) {
	if *field == "" {
		*field = os.Getenv(env)
	}
}
