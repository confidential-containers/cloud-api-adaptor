package e2e

import (
	"log"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
)

// the test will retrieve a kbs token to verify a successful remote attestation
func DoTestRemoteAttestation(t *testing.T, e env.Environment, assert CloudAssert, kbsEndpoint string) {
	name := "remote-attestation"
	image := "quay.io/curl/curl:latest"
	// fail on non 200 code, silent, but output on failure
	cmd := []string{"curl", "-f", "-s", "-S", "-o", "/dev/null", "http://127.0.0.1:8006/aa/token?token_type=kbs"}
	initdata, err := buildInitdataAnnotation(kbsEndpoint, testInitdata)
	if err != nil {
		log.Fatalf("failed to build initdata %s", err)
	}
	annotations := map[string]string{
		INITDATA_ANNOTATION: initdata,
	}
	job := NewJob(E2eNamespace, name, 0, image, WithJobCommand(cmd), WithJobAnnotations(annotations))
	NewTestCase(t, e, "RemoteAttestation", assert, "Received KBS token").WithJob(job).Run()
}
