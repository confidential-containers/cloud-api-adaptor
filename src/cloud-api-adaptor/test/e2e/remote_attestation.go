package e2e

import (
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
)

// the test will retrieve a kbs token to verify a successful remote attestation
func DoTestRemoteAttestation(t *testing.T, e env.Environment, assert CloudAssert) {
	name := "remote-attestation"
	image := "quay.io/curl/curl:latest"
	// fail on non 200 code, silent, but output on failure
	job := NewJob(E2eNamespace, name, 0, image, "curl", "-f", "-s", "-S", "-o", "/dev/null", "http://127.0.0.1:8006/aa/token?token_type=kbs")
	NewTestCase(t, e, "RemoteAttestation", assert, "Received KBS token").WithJob(job).Run()
}
