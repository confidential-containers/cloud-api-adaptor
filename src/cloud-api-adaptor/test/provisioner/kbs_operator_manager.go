// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// operatorKbsManager implements KbsManager for the trustee operator,
// where the KBS admin API is disabled. Resources are provisioned via k8s
// Secrets registered in the KbsConfig CR; policies via a ConfigMap.
//
// Configure with env vars:
//
//	KBS_NS                  Namespace (default: trustee-operator-system)
//	KBS_SVC_NAME            Service name (default: kbs-service)
//	KBS_DEPLOYMENT          Deployment name (default: trustee-deployment)
//	KBS_RESOURCE_POLICY_CM  ConfigMap for the resource release policy
//	                        (default: trustee-config-resource-policy)
//
// The KbsConfig CR name is discovered automatically.
type operatorKbsManager struct {
	ns               string
	svcName          string
	deploymentName   string
	resourcePolicyCM string

	kbsConfigCR    string // cached after first discovery
	endpoint       string // cached after first discovery
	addedSecrets   []string
	policyModified bool
}

// Common built-in policies for the operator backend (no trustee repo required).
// Keys match the filenames used by EnableKbsCustomized*Policy.
var builtinPolicies = map[string]string{
	"allow_all.rego": "package policy\ndefault allow = true\n",
	"deny_all.rego":  "package policy\ndefault allow = false\n",
}

// NewOperatorKbsManager returns a KbsManager for an operator-managed
// Trustee. Configuration comes from env vars; see operatorKbsManager.
func NewOperatorKbsManager() KbsManager {
	return newOperatorKbsManager()
}

func newOperatorKbsManager() *operatorKbsManager {
	return &operatorKbsManager{
		ns:               envOrDefault("KBS_NS", "trustee-operator-system"),
		svcName:          envOrDefault("KBS_SVC_NAME", "kbs-service"),
		deploymentName:   envOrDefault("KBS_DEPLOYMENT", "trustee-deployment"),
		resourcePolicyCM: envOrDefault("KBS_RESOURCE_POLICY_CM", "trustee-config-resource-policy"),
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// GetKbsEndpoint returns KBS_ENDPOINT env if set, otherwise discovers the
// ClusterIP service address (suitable for in-cluster or route-backed access).
func (p *operatorKbsManager) GetKbsEndpoint(_ context.Context, _ *envconf.Config) (string, error) {
	if ep := os.Getenv("KBS_ENDPOINT"); ep != "" {
		p.endpoint = ep
		return ep, nil
	}
	out, err := operatorRun("kubectl", "get", "svc", p.svcName, "-n", p.ns,
		"-o", "jsonpath={.spec.clusterIP}:{.spec.ports[0].port}")
	if err != nil {
		return "", fmt.Errorf("discovering KBS service: %w", err)
	}
	parts := strings.SplitN(strings.TrimSpace(out), ":", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", fmt.Errorf("unexpected service output: %q", out)
	}
	p.endpoint = fmt.Sprintf("http://%s:%s", parts[0], parts[1])
	return p.endpoint, nil
}

func (p *operatorKbsManager) GetCachedKbsEndpoint() (string, error) {
	if p.endpoint == "" {
		return "", fmt.Errorf("KBS endpoint not yet discovered; call GetKbsEndpoint first")
	}
	return p.endpoint, nil
}

// SetSecret stores data at a KBS resource path (repo/type/tag). Only
// repo=default is supported by the operator backend (kbsSecretResources).
func (p *operatorKbsManager) SetSecret(resourcePath string, data []byte) error {
	parts := strings.SplitN(resourcePath, "/", 3)
	if len(parts) != 3 {
		return fmt.Errorf("resource path must be repo/type/tag, got: %s", resourcePath)
	}
	if parts[0] != "default" {
		return fmt.Errorf("operator backend only supports repo=default, got: %s", parts[0])
	}
	return p.upsertResource(parts[1], parts[2], data)
}

func (p *operatorKbsManager) SetImageDecryptionKey(keyID string, key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("image decryption key must be exactly 32 bytes, got %d", len(key))
	}
	return p.SetSecret(keyID, key)
}

// EnableKbsCustomizedResourcePolicy sets the KBS resource release policy.
// Accepts a basename from the built-in set or a full file path.
func (p *operatorKbsManager) EnableKbsCustomizedResourcePolicy(policyFile string) error {
	rego, err := p.resolvePolicy(policyFile)
	if err != nil {
		return err
	}
	return p.applyResourcePolicy(rego)
}

// EnableKbsCustomizedAttestationPolicy is not yet implemented for the operator
// backend. Attestation policies require cluster-specific CR fields.
func (p *operatorKbsManager) EnableKbsCustomizedAttestationPolicy(policyFile string) error {
	return fmt.Errorf("attestation policy not yet supported by operator backend (file: %s)", policyFile)
}

// Delete is a no-op: KBS is pre-installed and managed by the operator.
func (p *operatorKbsManager) Delete(_ context.Context, _ *envconf.Config) error {
	return nil
}

// RevertResources restores the pre-test state by unregistering the secrets this
// manager added to the KbsConfig, deleting them, and restarting KBS. Resources
// that already existed before the test are left untouched.
func (p *operatorKbsManager) RevertResources() error {
	if len(p.addedSecrets) == 0 {
		return nil
	}
	cr, err := p.discoverKbsConfig()
	if err != nil {
		return err
	}

	remove := make(map[string]bool, len(p.addedSecrets))
	for _, s := range p.addedSecrets {
		remove[s] = true
	}

	current, err := operatorRun("kubectl", "get", "kbsconfig", cr, "-n", p.ns,
		"-o", "jsonpath={.spec.kbsSecretResources[*]}")
	if err != nil {
		return err
	}
	kept := []string{}
	for _, s := range strings.Fields(current) {
		if !remove[s] {
			kept = append(kept, s)
		}
	}
	list, _ := json.Marshal(kept)
	log.Infof("Unregistering %v from KbsConfig %s", p.addedSecrets, cr)
	if err := operatorRunDiscard("kubectl", "patch", "kbsconfig", cr, "-n", p.ns,
		"--type=merge",
		"-p", fmt.Sprintf(`{"spec":{"kbsSecretResources":%s}}`, list)); err != nil {
		return fmt.Errorf("unregistering secrets from KbsConfig: %w", err)
	}

	for s := range remove {
		if err := operatorRunDiscard("kubectl", "delete", "secret", s, "-n", p.ns,
			"--ignore-not-found"); err != nil {
			log.Warnf("deleting secret %s: %v", s, err)
		}
	}
	p.addedSecrets = nil
	return p.rollout()
}

// upsertResource creates/updates the k8s Secret and registers it in the
// KbsConfig CR's kbsSecretResources, restarting KBS if the secret is new.
func (p *operatorKbsManager) upsertResource(secretType, tag string, data []byte) error {
	f, err := os.CreateTemp("", "kbs-resource-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	f.Close()

	log.Infof("Upserting KBS secret %s/%s in %s", secretType, tag, p.ns)
	if err := operatorApply("kubectl", "create", "secret", "generic", secretType,
		"--from-file", tag+"="+f.Name(), "-n", p.ns,
		"--dry-run=client", "-o", "yaml"); err != nil {
		return fmt.Errorf("upserting secret %s: %w", secretType, err)
	}

	cr, err := p.discoverKbsConfig()
	if err != nil {
		return err
	}
	current, _ := operatorRun("kubectl", "get", "kbsconfig", cr, "-n", p.ns,
		"-o", "jsonpath={.spec.kbsSecretResources}")
	if strings.Contains(" "+current+" ", " "+secretType+" ") {
		// Already part of the pre-test state; leave it for RevertResources to keep.
		return nil
	}
	if err := operatorRunDiscard("kubectl", "patch", "kbsconfig", cr, "-n", p.ns,
		"--type=json",
		"-p", `[{"op":"add","path":"/spec/kbsSecretResources/-","value":"`+secretType+`"}]`); err != nil {
		return fmt.Errorf("registering secret in KbsConfig: %w", err)
	}
	// Track only what we newly registered so RevertResources restores exactly the
	// pre-test state.
	p.trackSecret(secretType)
	return p.rollout()
}

func (p *operatorKbsManager) trackSecret(name string) {
	for _, s := range p.addedSecrets {
		if s == name {
			return
		}
	}
	p.addedSecrets = append(p.addedSecrets, name)
}

// applyResourcePolicy writes the rego content to the policy ConfigMap and
// restarts KBS.
func (p *operatorKbsManager) applyResourcePolicy(rego string) error {
	f, err := os.CreateTemp("", "kbs-policy-*.rego")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(rego); err != nil {
		f.Close()
		return err
	}
	f.Close()

	log.Infof("Applying KBS resource policy from %s", f.Name())
	if err := operatorApply("kubectl", "create", "configmap", p.resourcePolicyCM,
		"--from-file=resource-policy.rego="+f.Name(), "-n", p.ns,
		"--dry-run=client", "-o", "yaml"); err != nil {
		return fmt.Errorf("applying resource policy: %w", err)
	}
	p.policyModified = true
	return p.rollout()
}

func (p *operatorKbsManager) discoverKbsConfig() (string, error) {
	if p.kbsConfigCR != "" {
		return p.kbsConfigCR, nil
	}
	out, err := operatorRun("kubectl", "get", "kbsconfig", "-n", p.ns,
		"-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		return "", fmt.Errorf("discovering KbsConfig CR: %w", err)
	}
	name := strings.TrimSpace(out)
	if name == "" {
		return "", fmt.Errorf("no KbsConfig found in namespace %s", p.ns)
	}
	p.kbsConfigCR = name
	return name, nil
}

func (p *operatorKbsManager) rollout() error {
	log.Infof("Restarting KBS deployment %s/%s", p.ns, p.deploymentName)
	if err := operatorRunDiscard("kubectl", "rollout", "restart",
		"deploy/"+p.deploymentName, "-n", p.ns); err != nil {
		return fmt.Errorf("rollout restart: %w", err)
	}
	return operatorRunDiscard("kubectl", "rollout", "status",
		"deploy/"+p.deploymentName, "-n", p.ns, "--timeout=120s")
}

func (p *operatorKbsManager) resolvePolicy(policyFile string) (string, error) {
	if content, ok := builtinPolicies[filepath.Base(policyFile)]; ok {
		return content, nil
	}
	b, err := os.ReadFile(policyFile)
	if err != nil {
		return "", fmt.Errorf("reading policy file %s: %w", policyFile, err)
	}
	return string(b), nil
}

// operatorRun runs a command and returns trimmed stdout.
func operatorRun(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %v: %s", name, args, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func operatorRunDiscard(name string, args ...string) error {
	_, err := operatorRun(name, args...)
	return err
}

// operatorApply pipes "kubectl <args>" into "kubectl apply -f -".
func operatorApply(name string, args ...string) error {
	create := exec.Command(name, args...)
	var buf bytes.Buffer
	create.Stdout = &buf
	create.Stderr = os.Stderr
	if err := create.Run(); err != nil {
		return err
	}
	apply := exec.Command("kubectl", "apply", "-f", "-")
	apply.Stdin = &buf
	apply.Stdout = os.Stdout
	apply.Stderr = os.Stderr
	return apply.Run()
}
