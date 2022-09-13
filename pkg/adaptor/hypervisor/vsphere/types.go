package vsphere

import (
	"net"
)

type Config struct {
	VcenterURL   string
	UserName     string
	Password     string
	Insecure     bool
	Datacenter   string
	Vcluster     string
	Datastore    string
	Resourcepool string
	Deployfolder string
	Template     string // template will be hardcoded podvm-template for now
}

type createInstanceOutput struct {
	uuid string
	ips  []net.IP
}
