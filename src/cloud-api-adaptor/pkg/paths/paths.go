package paths

const (
	AACfgPath        = "/run/peerpod/aa.toml"
	AuthFilePath     = "/run/peerpod/auth.json"
	CDHCfgPath       = "/run/peerpod/cdh.toml"
	InitDataPath     = "/run/peerpod/initdata"
	AgentCfgPath     = "/run/peerpod/agent-config.toml"
	ForwarderCfgPath = "/run/peerpod/daemon.json"
	// This is not mounted under /run to avoid conflicting with
	// docker provider mounts.
	UserDataPath = "/var/tmp/media/cidata/user-data"
)
