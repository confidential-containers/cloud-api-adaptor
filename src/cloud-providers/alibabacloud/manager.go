// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package alibabacloud

import (
	"flag"
	"os"
	"strings"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var alibabacloudcfg Config

type Manager struct{}

func init() {
	provider.AddCloudProvider("alibabacloud", &Manager{})
}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {
	flags.StringVar(&alibabacloudcfg.AccessKeyId, "alibabacloud-access-key-id", "", "Access Key ID, defaults to `ALIBABACLOUD_ACCESS_KEY_ID`")
	flags.StringVar(&alibabacloudcfg.SecretKey, "alibabacloud-secret-access-key", "", "Secret Key, defaults to `ALIBABACLOUD_SECRET_ACCESS_KEY`")
	flags.StringVar(&alibabacloudcfg.Region, "region", "", "Region")
	flags.StringVar(&alibabacloudcfg.ImageId, "imageid", "", "Pod VM image id")
	flags.StringVar(&alibabacloudcfg.InstanceType, "instance-type", "ecs.g8i.xlarge", "Pod VM instance type")
	flags.Var(&alibabacloudcfg.SecurityGroupIds, "security-group-ids", "Security Group Ids to be used for the Pod VM, comma separated")
	flags.StringVar(&alibabacloudcfg.KeyName, "keyname", "", "SSH Keypair name to be used with the Pod VM")
	flags.StringVar(&alibabacloudcfg.VpcId, "vpc-id", "", "VPC ID to be used for the Pod VMs")
	flags.StringVar(&alibabacloudcfg.VswitchId, "vswitch-id", "", "vSwitch ID to be used for the Pod VMs")
	// Add a key value list parameter to indicate custom tags to be used for the Pod VMs
	flags.Var(&alibabacloudcfg.Tags, "tags", "Custom tags (key=value pairs) to be used for the Pod VMs, comma separated")
	flags.BoolVar(&alibabacloudcfg.UsePublicIP, "use-public-ip", false, "Use Public IP for connecting to the kata-agent inside the Pod VM")
	// Add a parameter to indicate the root volume size for the Pod VMs
	// Default is 40GiBs for free tier. Hence use it as default
	flags.IntVar(&alibabacloudcfg.SystemDiskSize, "system-disk-size", 40, "System Disk size (in GiB) for the Pod VMs")
	flags.BoolVar(&alibabacloudcfg.DisableCVM, "disable-cvm", false, "Use non-CVMs for peer pods")
}

func (_ *Manager) LoadEnv() {
	provider.DefaultToEnv(&alibabacloudcfg.AccessKeyId, "ALIBABACLOUD_ACCESS_KEY_ID", "")
	provider.DefaultToEnv(&alibabacloudcfg.SecretKey, "ALIBABACLOUD_ACCESS_KEY_SECRET", "")
	provider.DefaultToEnv(&alibabacloudcfg.Region, "REGION", "cn-beijing")
	provider.DefaultToEnv(&alibabacloudcfg.ImageId, "IMAGEID", "")
	provider.DefaultToEnv(&alibabacloudcfg.InstanceType, "PODVM_INSTANCE_TYPE", "ecs.g8i.xlarge")
	provider.DefaultToEnv(&alibabacloudcfg.VswitchId, "VSWITCH_ID", "")
	if len(alibabacloudcfg.SecurityGroupIds) == 0 {
		envVal := os.Getenv("SECURITY_GROUP_IDS")
		var val []string
		if envVal == "" {
			val = []string{"cn-beijing"}
		} else {
			val = strings.Split(envVal, ",")
		}

		alibabacloudcfg.SecurityGroupIds = val
	}
	if len(alibabacloudcfg.Tags) == 0 {
		envVal := os.Getenv("TAGS")
		val := make(map[string]string)
		if envVal != "" {
			pairs := strings.Split(envVal, ",")
			for _, p := range pairs {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				kv := strings.SplitN(p, "=", 2)
				if len(kv) != 2 {
					continue
				}
				k := strings.TrimSpace(kv[0])
				v := strings.TrimSpace(kv[1])
				if k == "" {
					continue
				}
				val[k] = v
			}
		}

		alibabacloudcfg.Tags = val
	}
	provider.DefaultToEnv(&alibabacloudcfg.KeyName, "KEYNAME", "")
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&alibabacloudcfg)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &alibabacloudcfg
}
