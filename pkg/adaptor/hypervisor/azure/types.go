package azure

type Config struct {
	SubscriptionId    string
	ClientId          string
	ClientSecret      string
	TenantId          string
	ResourceGroupName string
	Zone              string
	Region            string
	SubnetName        string
	SubnetId          string
	VnetName          string
	SecurityGroupName string
	SecurityGroupId   string
	Size              string
	ImageId           string
	SSHKeyPath        string
}
