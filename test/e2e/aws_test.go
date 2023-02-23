//go:build aws

package e2e

import (
	"testing"
)

// AWSAssert implements the CloudAssert interface.
type AWSAssert struct {
}

func (aa AWSAssert) HasPodVM(t *testing.T, id string) {

}

func TestAWSCreateSimplePod(t *testing.T) {

}
