package aws

type Config struct {
        ApiKey                   string
        ProfileName              string
        ZoneName                 string
        ImageID                  string
        PrimarySubnetID          string
        PrimarySecurityGroupID   string
        SecondarySubnetID        string
        SecondarySecurityGroupID string
        KeyID                    string
        VpcID                    string
}
