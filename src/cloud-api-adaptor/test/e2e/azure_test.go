//go:build azure

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/initdata"
	_ "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/azure"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func TestBasicAzureDeletePod(t *testing.T) {
	t.Parallel()
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestBasicAzureCreateSimplePod(t *testing.T) {
	t.Parallel()
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestBasicAzureCreatePodWithConfigMap(t *testing.T) {
	t.Parallel()
	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestBasicAzureCreatePodWithSecret(t *testing.T) {
	t.Parallel()
	DoTestCreatePodWithSecret(t, testEnv, assert)
}

func TestBasicAzureCreateNginxDeployment(t *testing.T) {
	t.Parallel()
	DoTestNginxDeployment(t, testEnv, assert)
}

func TestNetAzurePodToServiceCommunication(t *testing.T) {
	t.Parallel()
	DoTestPodToServiceCommunication(t, testEnv, assert)
}

func TestNetAzurePodsMTLSCommunication(t *testing.T) {
	t.Parallel()
	DoTestPodsMTLSCommunication(t, testEnv, assert)
}

func TestResAzurePodVMwithAnnotationsInstanceType(t *testing.T) {
	SkipTestOnCI(t)
	t.Parallel()
	instanceSize := "Standard_DC2as_v5"
	DoTestPodVMwithAnnotationsInstanceType(t, testEnv, assert, instanceSize)
}

func TestResAzurePodVMwithAnnotationsInvalidInstanceType(t *testing.T) {
	t.Parallel()
	// Using an instance type that's not configured in the AZURE_INSTANCE_SIZE
	instanceSize := "Standard_D8as_v5"
	DoTestPodVMwithAnnotationsInvalidInstanceType(t, testEnv, assert, instanceSize)
}

// Test with device annotation
func TestSecAzurePodWithCrioDeviceAnnotation(t *testing.T) {
	if !isTestOnCrio() {
		t.Skip("Skipping test as it is not running on CRI-O")
	}
	t.Parallel()
	DoTestPodWithCrioDeviceAnnotation(t, testEnv, assert)
}

// Negative test with device annotation
func TestSecAzurePodWithIncorrectDeviceAnnotation(t *testing.T) {
	if !isTestOnCrio() {
		t.Skip("Skipping test as it is not running on CRI-O")
	}
	t.Parallel()
	DoTestPodWithIncorrectCrioDeviceAnnotation(t, testEnv, assert)
}

// Test with init container
func TestBasicAzurePodWithInitContainer(t *testing.T) {
	t.Parallel()
	DoTestPodWithInitContainer(t, testEnv, assert)
}

// Test to check the presence if pod can access files from internet
// Use DoTestPodWithSpecificCommands and provide the commands to be executed in the pod
func TestNetAzurePodToDownloadExternalFile(t *testing.T) {
	t.Parallel()
	// Create TestCommand struct with the command to download index.html
	command1 := TestCommand{
		Command:             []string{"wget", "-q", "www.google.com"},
		TestCommandStdoutFn: IsBufferEmpty,
		TestCommandStderrFn: IsBufferEmpty,
	}

	// Check index.html is downloaded
	command2 := TestCommand{
		Command: []string{"ls", "index.html"},
		TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
			if strings.Contains(stdout.String(), "index.html") {
				t.Logf("index.html is present in the pod")
				return true
			} else {
				t.Logf("index.html is not present in the pod")
				return false
			}
		},
		TestCommandStderrFn: IsBufferEmpty,
	}

	commands := []TestCommand{command1, command2}

	DoTestPodWithSpecificCommands(t, testEnv, assert, commands)
}

// Method to check external IP access using ping
func TestNetAzureCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
	SkipTestOnCI(t)
	t.Parallel()
	DoTestCreatePeerPodContainerWithExternalIPAccess(t, testEnv, assert)
}

func TestConfAzureKbsKeyRelease(t *testing.T) {
	if !isTestWithKbs() {
		t.Skip("Skipping kbs related test as kbs is not deployed")
	}
	t.Parallel()
	kbsEndpoint, _ := keyBrokerService.GetCachedKbsEndpoint()
	testSecret := envconf.RandomName("coco-pp-e2e-secret", 25)
	resourcePath := "caa/workload_key/test_key.bin"
	err := keyBrokerService.SetSecret(resourcePath, []byte(testSecret))
	if err != nil {
		t.Fatalf("SetSecret failed with: %v", err)
	}
	DoTestKbsKeyRelease(t, testEnv, assert, kbsEndpoint, resourcePath, testSecret)
}

func TestConfAzureRemoteAttestation(t *testing.T) {
	t.Parallel()
	var kbsEndpoint string
	if ep := os.Getenv("KBS_ENDPOINT"); ep != "" {
		kbsEndpoint = ep
	} else if keyBrokerService == nil {
		t.Skip("Skipping because KBS config is missing")
	} else {
		var err error
		kbsEndpoint, err = keyBrokerService.GetCachedKbsEndpoint()
		if err != nil {
			t.Fatalf("GetCachedKbsEndpoint failed with: %v", err)
		}
	}
	DoTestRemoteAttestation(t, testEnv, assert, kbsEndpoint)
}

func TestConfAzureTrusteeOperatorKeyReleaseForSpecificKey(t *testing.T) {
	if !isTestWithTrusteeOperator() {
		t.Skip("Skipping kbs related test as Trustee Operator is not deployed")
	}
	t.Parallel()
	kbsEndpoint, err := keyBrokerService.GetCachedKbsEndpoint()
	if err != nil {
		t.Fatalf("GetCachedKbsEndpoint failed with: %v", err)
	}
	DoTestKbsKeyRelease(t, testEnv, assert, kbsEndpoint, "default/kbsres1/key1", "res1val1")
}

func TestConfAzureImageDecryption(t *testing.T) {
	if !isTestWithKbs() {
		t.Skip("Skipping kbs related test as kbs is not deployed")
	}
	t.Parallel()

	DoTestImageDecryption(t, testEnv, assert, keyBrokerService)
}

// This test is to verify that the initdata is measured correctly. The digest algorith in the initdata fixture
// is sha384. The initdata spec requires the digest to be truncated/padded to the TEE's requirement. In this case,
// the az tpm attester requires the digest to be sha256 and is hence truncated
func TestConfAzureInitDataMeasurement(t *testing.T) {
	kbsEndpoint := "http://some.endpoint"
	annotation, err := buildInitdataAnnotation(kbsEndpoint, testInitdata)
	if err != nil {
		log.Fatalf("failed to build initdata %s", err)
	}

	decoded, err := initdata.DecodeAnnotation(annotation)
	if err != nil {
		log.Fatalf("failed to decode initdata %s", err)
	}
	digest := sha512.Sum384(decoded)
	truncatedDigest := digest[:32]
	zeroes := bytes.Repeat([]byte{0x00}, 32)

	hasher := sha256.New()
	hasher.Write(zeroes)
	hasher.Write(truncatedDigest)
	msmt := hasher.Sum(nil)

	name := "initdata-msmt"
	image := "quay.io/confidential-containers/test-images:curl-jq"

	// truncate the measurement to 32 bytes
	strValues := make([]string, len(msmt))
	for i, v := range msmt {
		strValues[i] = strconv.Itoa(int(v))
	}
	// json array string
	msStr := "[" + strings.Join(strValues, ",") + "]"

	shCmd := "curl -s \"http://127.0.0.1:8006/aa/evidence?runtime_data=test\" | jq -c '(.quote // .tpm_quote).pcrs[8]'"
	cmd := []string{"sh", "-c", shCmd}

	annotations := map[string]string{
		INITDATA_ANNOTATION: annotation,
	}
	job := NewJob(E2eNamespace, name, 0, image, WithJobCommand(cmd), WithJobAnnotations(annotations))
	NewTestCase(t, testEnv, "InitDataMeasurement", assert, "InitData measured correctly").WithJob(job).WithExpectedPodLogString(msStr).Run()
}
