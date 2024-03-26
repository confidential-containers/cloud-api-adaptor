package provider

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"plugin"
	"strings"
)

type CloudProvider interface {
	ParseCmd(flags *flag.FlagSet)
	LoadEnv()
	NewProvider() (Provider, error)
}

var providerTable map[string]CloudProvider = make(map[string]CloudProvider)

func getFileNameAndSha256sum(providerPath string) (string, string, error) {
	file, err := os.Open(providerPath)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	// Get the base filename without the directory path
	filename := filepath.Base(providerPath)

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return filename, "", err
	}
	sum := hash.Sum(nil)
	return filename, fmt.Sprintf("%x", sum), nil
}

func hasExecutePermission(providerPath string) (bool, error) {
	// Get the parent directory of the specified file path
	dir := filepath.Dir(providerPath)

	// Stat the directory to get its file info
	dirInfo, err := os.Stat(dir)
	if err != nil {
		return false, err
	}

	// Check if the directory has execute permission for the current user
	mode := dirInfo.Mode()
	executePermission := mode&os.ModeDir != 0 && mode&0100 != 0

	return executePermission, nil
}

// LoadCloudProvider loads cloud provider external plugin from the given path CLOUD_PROVIDER_EXTERNAL_PLUGIN_PATH
// The values of 1) ${CLOUD_PROVIDER}, 2) the filename of the cloud provider external plugin
// and 3) the provider defined within the external plugin must all match
func LoadCloudProvider(name string) {
	if os.Getenv("ENABLE_CLOUD_PROVIDER_EXTERNAL_PLUGIN") != "true" {
		logger.Printf("Cloud provider external plugin loading is disabled, skipping plugin loading")
		return
	}
	externalPluginPath := os.Getenv("CLOUD_PROVIDER_EXTERNAL_PLUGIN_PATH")
	executePermission, err := hasExecutePermission(externalPluginPath)
	if err != nil {
		logger.Printf("Failed to retrieve file information for the parent directory of CLOUD_PROVIDER_EXTERNAL_PLUGIN_PATH %s", err)
		return
	}
	if !executePermission {
		logger.Printf("The parent directory of the external plugin %s lacks execute permissions", filepath.Dir(externalPluginPath))
		return
	}
	cloudProviderPluginHash := os.Getenv("CLOUD_PROVIDER_EXTERNAL_PLUGIN_HASH")
	if externalPluginPath == "" {
		logger.Printf("Env CLOUD_PROVIDER_EXTERNAL_PLUGIN_PATH is not set")
		return
	}
	if cloudProviderPluginHash == "" {
		logger.Printf("Env CLOUD_PROVIDER_EXTERNAL_PLUGIN_HASH is not set")
		return
	}
	filename, realPluginHash, err := getFileNameAndSha256sum(externalPluginPath)
	if !strings.EqualFold(filename, name+".so") {
		logger.Printf("Filename of the external plugin: %s, is not match with CLOUD_PROVIDER: %s", filename, name)
		return
	}
	logger.Printf("Loading external plugin %s from %s", name, externalPluginPath)

	if err != nil {
		logger.Printf("Failed to calculate the SHA256 checksum of the external plugin %s", err)
		return
	}
	if cloudProviderPluginHash != realPluginHash {
		logger.Printf("The sha256sum of the external plugin: %s doesn't match the one from configmap: %s", realPluginHash, cloudProviderPluginHash)
		return
	}
	_, err = plugin.Open(externalPluginPath)
	if err != nil {
		logger.Printf("Failed to open the external plugin %s", err)
	} else {
		logger.Printf("Successfully opened the external plugin %s", externalPluginPath)
	}
}

func Get(name string) CloudProvider {
	LoadCloudProvider(name)
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
