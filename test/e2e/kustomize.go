package e2e

import (
	"bytes"
	"context"
	"io"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

type KustomizeHelper struct {
	configDir string // path to configuration directory. For example, an overlay directory.
}

// Apply builds the configuration directory and deploy the resulted manifest.
func (kh *KustomizeHelper) Apply(ctx context.Context, cfg *envconf.Config) error {
	reader, err := kh.BuildAsYaml()
	if err != nil {
		return err
	}
	client, err := cfg.NewClient()
	if err != nil {
		return err
	}

	err = decoder.DecodeEach(ctx, reader, decoder.CreateIgnoreAlreadyExists(client.Resources()))
	if err != nil {
		return err
	}
	return nil
}

// BuildAsYaml only build the overlay directory and returns the manifest as the YAML representation.
func (kh *KustomizeHelper) BuildAsYaml() (io.Reader, error) {
	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	fsys := filesys.MakeFsOnDisk()
	resourcesMap, err := k.Run(fsys, kh.configDir)
	if err != nil {
		return nil, err
	}
	yml, err := resourcesMap.AsYaml()
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(yml), nil
}

// Delete builds the overlay directory and delete the resulted resources.
func (kh *KustomizeHelper) Delete(ctx context.Context, cfg *envconf.Config) error {
	reader, err := kh.BuildAsYaml()
	if err != nil {
		return err
	}
	client, err := cfg.NewClient()
	if err != nil {
		return err
	}

	err = decoder.DecodeEach(ctx, reader, decoder.DeleteHandler(client.Resources()))
	if err != nil {
		return err
	}
	return nil
}
