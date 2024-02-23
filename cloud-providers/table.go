package provider

import (
	"flag"
)

type CloudProvider interface {
	ParseCmd(flags *flag.FlagSet)
	LoadEnv()
	NewProvider() (Provider, error)
}

var providerTable map[string]CloudProvider = make(map[string]CloudProvider)

func Get(name string) CloudProvider {
	return providerTable[name]
}

func AddCloudProvider(name string, cloud CloudProvider) {
	providerTable[name] = cloud
}

func List() []string {

	var list []string

	for name := range providerTable {
		list = append(list, name)
	}

	return list
}
