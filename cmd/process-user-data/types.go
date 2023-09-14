package main

const (
	programName   = "process-user-data"
	providerAzure = "azure"
	providerAws   = "aws"

	defaultAgentConfigPath  = "/etc/agent-config.toml"
	defaultAuthJsonFilePath = "/etc/auth.json"
	offlineKbcAuthFile      = "/etc/aa-offline_fs_kbc-resources.json"
)

type Config struct {
	daemonConfigPath     string
	agentConfigPath      string
	userData             string
	userDataFetchTimeout int
}

type Endpoints struct {
	Allowed []string `toml:"allowed"`
}

type AgentConfig struct {
	EnableSignatureVerification bool      `toml:"enable_signature_verification"`
	ServerAddr                  string    `toml:"server_addr"`
	AaKbcParams                 string    `toml:"aa_kbc_params"`
	ImageRegistryAuthFile       string    `toml:"image_registry_auth_file"`
	Endpoints                   Endpoints `toml:"endpoints"`
}
