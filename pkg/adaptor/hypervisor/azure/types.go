package azure

import "github.com/confidential-containers/cloud-api-adaptor/pkg/util"

type Config struct {
	SubscriptionId    string
	ClientId          string
	ClientSecret      string
	TenantId          string
	ResourceGroupName string
	Zone              string
	Region            string
	SubnetId          string
	SecurityGroupName string
	SecurityGroupId   string
	Size              string
	ImageId           string
	SSHKeyPath        string
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "ClientId", "TenantId", "ClientSecret").(*Config)
}
