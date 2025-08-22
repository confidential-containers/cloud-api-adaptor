package provisioner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"golang.org/x/exp/slices"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/kustomize/pkg/commands/kustfile"
	"sigs.k8s.io/kustomize/pkg/fs"
	"sigs.k8s.io/kustomize/pkg/image"
	"sigs.k8s.io/kustomize/pkg/patch"
	ktypes "sigs.k8s.io/kustomize/pkg/types"
)

type KustomizeOverlay struct {
	ConfigDir string // path to the overlay directory
	Yaml      []byte // Resources built in YAML
}

func NewKustomizeOverlay(dir string) (*KustomizeOverlay, error) {
	yml, err := BuildKustomizeOverlayAsYaml(dir)
	if err != nil {
		return nil, err
	}

	return &KustomizeOverlay{
		ConfigDir: dir,
		Yaml:      yml,
	}, nil
}

// Apply builds the configuration directory and deploy the resulted manifest.
func (kh *KustomizeOverlay) Apply(ctx context.Context, cfg *envconf.Config) error {
	client, err := cfg.NewClient()
	if err != nil {
		return err
	}

	err = decoder.DecodeEach(ctx, bytes.NewReader(kh.Yaml), decoder.CreateIgnoreAlreadyExists(client.Resources()))
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

	err = decoder.DecodeEach(ctx, bytes.NewReader(kh.Yaml), decoder.DeleteHandler(client.Resources()))
	if err != nil {
		return err
	}
	return nil
}

func (kh *KustomizeOverlay) YamlReload() error {
	yml, err := BuildKustomizeOverlayAsYaml(kh.ConfigDir)
	if err != nil {
		return err
	}
	kh.Yaml = yml

	return nil
}

// SetKustomizeConfigMapGeneratorLiteral updates the kustomization YAML by setting `value` to `key` on the
// `cmgName` ConfigMapGenerator literals. If `key` does not exist then a new entry is added.
func (kh *KustomizeOverlay) SetKustomizeConfigMapGeneratorLiteral(cmgName string, key string, value string) (err error) {
	// Unfortunately NewKustomizationFile() needs the work directory (wd) be the overlay directory,
	// otherwise it won't find the kustomize yaml. So let's save the current wd then switch back when
	// we are done.
	oldwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err = os.Chdir(kh.ConfigDir); err != nil {
		return err
	}
	defer func() {
		err = os.Chdir(oldwd)
	}()

	// Unfortunately (2) the kustomizationFile struct is not exported by the package so reading operation
	// cannot be refactored in a separate function.
	kf, err := kustfile.NewKustomizationFile(fs.MakeRealFS())
	if err != nil {
		return err
	}

	m, err := kf.Read()
	if err != nil {
		return err
	}

	if err = setConfigMapGeneratorLiteral(m, cmgName, key, value); err != nil {
		return err
	}

	if err = kf.Write(m); err != nil {
		return err
	}

	return nil
}

// SetKustomizeSecretGeneratorLiteral updates the kustomization YAML by setting `value` to `key` on the
// `secretName` SecretGenerator literals. If `key` does not exist then a new entry is added.
func (kh *KustomizeOverlay) SetKustomizeSecretGeneratorLiteral(secretName string, key string, value string) (err error) {
	// TODO
	// Unfortunately NewKustomizationFile() needs the work directory (wd) be the overlay directory,
	// otherwise it won't find the kustomize yaml. So let's save the current wd then switch back when
	// we are done.
	oldwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err = os.Chdir(kh.ConfigDir); err != nil {
		return err
	}
	defer func() {
		err = os.Chdir(oldwd)
	}()

	// Unfortunately (2) the kustomizationFile struct is not exported by the package so reading operation
	// cannot be refactored in a separate function.
	kf, err := kustfile.NewKustomizationFile(fs.MakeRealFS())
	if err != nil {
		return err
	}

	m, err := kf.Read()
	if err != nil {
		return err
	}

	if err = setSecretGeneratorLiteral(m, secretName, key, value); err != nil {
		return err
	}

	if err = kf.Write(m); err != nil {
		return err
	}

	return nil
}

// SetKustomizeSecretGeneratorEnvs updates the kustomization YAML by adding the `env` on the
// `sgName` SecretGenerator env file.
func (kh *KustomizeOverlay) SetKustomizeSecretGeneratorEnv(sgName string, file string) (err error) {
	oldwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err = os.Chdir(kh.ConfigDir); err != nil {
		return err
	}
	defer func() {
		err = os.Chdir(oldwd)
	}()

	kf, err := kustfile.NewKustomizationFile(fs.MakeRealFS())
	if err != nil {
		return err
	}

	m, err := kf.Read()
	if err != nil {
		return err
	}

	if len(m.SecretGenerator) == 0 {
		m.SecretGenerator = append(m.SecretGenerator, ktypes.SecretArgs{
			GeneratorArgs: ktypes.GeneratorArgs{
				DataSources: ktypes.DataSources{
					EnvSource: file,
				},
			},
		})
	} else {
		i := slices.IndexFunc(m.SecretGenerator, func(sa ktypes.SecretArgs) bool { return sa.Name == sgName })
		if i == -1 {
			return fmt.Errorf("SecretGenerator %s not found", sgName)
		}
		gs := &m.SecretGenerator[i]
		if !stringSliceContains(gs.FileSources, file) {
			gs.EnvSource = file
		}

	}

	if err = kf.Write(m); err != nil {
		return err
	}

	return nil
}

// SetKustomizeSecretGeneratorFile updates the kustomization YAML by adding the `file` on the
// `sgName` SecretGenerator files.
func (kh *KustomizeOverlay) SetKustomizeSecretGeneratorFile(sgName string, file string) (err error) {
	oldwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err = os.Chdir(kh.ConfigDir); err != nil {
		return err
	}
	defer func() {
		err = os.Chdir(oldwd)
	}()

	kf, err := kustfile.NewKustomizationFile(fs.MakeRealFS())
	if err != nil {
		return err
	}

	m, err := kf.Read()
	if err != nil {
		return err
	}

	if len(m.SecretGenerator) == 0 {
		m.SecretGenerator = append(m.SecretGenerator, ktypes.SecretArgs{
			GeneratorArgs: ktypes.GeneratorArgs{
				DataSources: ktypes.DataSources{
					FileSources: []string{file},
				},
			},
		})
	} else {
		i := slices.IndexFunc(m.SecretGenerator, func(sa ktypes.SecretArgs) bool { return sa.Name == sgName })
		if i == -1 {
			return fmt.Errorf("SecretGenerator %s not found", sgName)
		}
		gs := &m.SecretGenerator[i]
		if !stringSliceContains(gs.FileSources, file) {
			gs.FileSources = append(gs.FileSources, file)
		}

	}

	if err = kf.Write(m); err != nil {
		return err
	}

	return nil
}

func (kh *KustomizeOverlay) AddToPatchesStrategicMerge(fileName string) error {
	oldwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err = os.Chdir(kh.ConfigDir); err != nil {
		return err
	}
	defer func() {
		err = os.Chdir(oldwd)
	}()

	kf, err := kustfile.NewKustomizationFile(fs.MakeRealFS())
	if err != nil {
		return err
	}

	m, err := kf.Read()
	if err != nil {
		return err
	}

	m.PatchesStrategicMerge = append(m.PatchesStrategicMerge, patch.StrategicMerge(fileName))

	return kf.Write(m)
}

func stringSliceContains(slice []string, value string) bool {
	for _, s := range slice {
		if s == value {
			return true
		}
	}
	return false
}

// SetKustomizeImage updates the kustomization YAML by setting `value` to `key` on the
// `Image`. If `key` does not exist then a new entry is added.
func (kh *KustomizeOverlay) SetKustomizeImage(imageName string, key string, value string) (err error) {
	if !isValidImageKey(key) {
		return fmt.Errorf("not supported image key: %s", key)
	}
	// TODO
	// Unfortunately NewKustomizationFile() needs the work directory (wd) be the overlay directory,
	// otherwise it won't find the kustomize yaml. So let's save the current wd then switch back when
	// we are done.
	oldwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err = os.Chdir(kh.ConfigDir); err != nil {
		return err
	}
	defer func() {
		err = os.Chdir(oldwd)
	}()

	// Unfortunately (2) the kustomizationFile struct is not exported by the package so reading operation
	// cannot be refactored in a separate function.
	kf, err := kustfile.NewKustomizationFile(fs.MakeRealFS())
	if err != nil {
		return err
	}

	m, err := kf.Read()
	if err != nil {
		return err
	}

	if len(m.Images) == 0 {
		return fmt.Errorf("no Image found")
	}
	i := slices.IndexFunc(m.Images, func(im image.Image) bool { return im.Name == imageName })
	if i == -1 {
		return fmt.Errorf("image %s not found", imageName)
	}
	image := &m.Images[i]

	switch key {
	case "newName":
		image.NewName = value
	case "newTag":
		image.NewTag = value
	case "digest":
		image.Digest = value
	default:
	}

	if err = kf.Write(m); err != nil {
		return err
	}

	return nil
}

func isValidImageKey(key string) bool {
	switch key {
	case "newName":
		return true
	case "newTag":
		return true
	case "digest":
		return true
	default:
		return false
	}
}

func setLiteral(literals []string, key string, value string) []string {
	newLiterals := literals
	newVal := fmt.Sprintf("%s=\"%s\"", key, value)

	// Find and replace the literal...
	var i int
	if i = slices.IndexFunc(newLiterals,
		func(l string) bool { return strings.HasPrefix(l, key+"=") }); i != -1 {
		newLiterals[i] = newVal
		return newLiterals
	} else {
		// ...or add a new literal
		return append(newLiterals, newVal)
	}
}

func setConfigMapGeneratorLiteral(k *ktypes.Kustomization, cmgName string, key string, value string) error {
	if len(k.ConfigMapGenerator) == 0 {
		return fmt.Errorf("no ConfigMapGenerator found")
	}

	i := slices.IndexFunc(k.ConfigMapGenerator, func(cma ktypes.ConfigMapArgs) bool { return cma.Name == cmgName })
	if i == -1 {
		return fmt.Errorf("ConfigMapGenerator %s not found", cmgName)
	}
	cmg := &k.ConfigMapGenerator[i]

	newLiterals := setLiteral(cmg.LiteralSources, key, value)
	cmg.LiteralSources = newLiterals

	return nil
}

func setSecretGeneratorLiteral(k *ktypes.Kustomization, secretName string, key string, value string) error {
	if len(k.SecretGenerator) == 0 {
		return fmt.Errorf("no SecretGenerator found")
	}

	i := slices.IndexFunc(k.SecretGenerator, func(sa ktypes.SecretArgs) bool { return sa.Name == secretName })
	if i == -1 {
		return fmt.Errorf("SecretGenerator %s not found", secretName)
	}
	secretg := &k.SecretGenerator[i]

	newLiterals := setLiteral(secretg.LiteralSources, key, value)
	secretg.LiteralSources = newLiterals

	return nil
}
