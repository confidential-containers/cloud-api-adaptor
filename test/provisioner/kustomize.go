package provisioner

import (
	"bytes"
	"context"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

type KustomizeOverlay struct {
	configDir string // path to the overlay directory
	yaml      []byte // Resources built in YAML
}

func NewKustomizeOverlay(dir string) (*KustomizeOverlay, error) {
	yml, err := BuildKustomizeOverlayAsYaml(dir)
	if err != nil {
		return nil, err
	}

	return &KustomizeOverlay{
		configDir: dir,
		yaml:      yml,
	}, nil
}

// Apply builds the configuration directory and deploy the resulted manifest.
func (kh *KustomizeOverlay) Apply(ctx context.Context, cfg *envconf.Config) error {
	client, err := cfg.NewClient()
	if err != nil {
		return err
	}

	err = decoder.DecodeEach(ctx, bytes.NewReader(kh.yaml), decoder.CreateIgnoreAlreadyExists(client.Resources()))
	if err != nil {
		return err
	}
	return nil
}

// BuildAsYaml only build the overlay directory and returns the manifest as the YAML representation.
func BuildKustomizeOverlayAsYaml(overlayDir string) ([]byte, error) {
	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	fsys := filesys.MakeFsOnDisk()
	resourcesMap, err := k.Run(fsys, overlayDir)
	if err != nil {
		return nil, err
	}
	yml, err := resourcesMap.AsYaml()
	if err != nil {
		return nil, err
	}
	return yml, nil
}

// Delete builds the overlay directory and delete the resulted resources.
func (kh *KustomizeOverlay) Delete(ctx context.Context, cfg *envconf.Config) error {
	client, err := cfg.NewClient()
	if err != nil {
		return err
	}

	err = decoder.DecodeEach(ctx, bytes.NewReader(kh.yaml), decoder.DeleteHandler(client.Resources()))
	if err != nil {
		return err
	}
	return nil
}

func (kh *KustomizeOverlay) YamlReload() error {
	yml, err := BuildKustomizeOverlayAsYaml(kh.configDir)
	if err != nil {
		return err
	}
	kh.yaml = yml

	return nil
}
