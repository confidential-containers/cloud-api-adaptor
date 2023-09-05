package main

const (
	programName            = "process-user-data"
	AzureImdsUrl           = "http://169.254.169.254/metadata/instance/compute?api-version=2021-01-01"
	AzureUserDataImdsUrl   = "http://169.254.169.254/metadata/instance/compute/userData?api-version=2021-01-01&format=text"
	defaultAgentConfigPath = "/etc/agent-config.toml"
)

type Config struct {
	daemonConfigPath     string
	agentConfigPath      string
	userData             string
	userDataFetchTimeout int
}
