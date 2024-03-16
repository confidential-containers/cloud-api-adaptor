package docker

type Config struct {
	DockerHost       string
	DockerAPIVersion string
	DockerCertPath   string
	DockerTLSVerify  bool
	DataDir          string
}
