//go:build aws

package e2e

import (
	"testing"
)

func TestAws(t *testing.T) {
	//
	testEnv.Test(t)
}
