package kubemgr

import (
	"slices"
	"testing"
)

func TestSecrets(t *testing.T) {
	InitKubeMgrMock()

	privateKey1, publicKey1, err1 := KubeMgr.CreateSecret("XYZ")
	if err1 != nil {
		t.Error(err1)
	}

	privateKey2, publicKey2, err2 := KubeMgr.ReadSecret("XYZ")
	if err1 != nil {
		t.Error(err2)
	}
	_, _, err3 := KubeMgr.ReadSecret("ABC")
	if err3 == nil {
		t.Error("Expected error")
	}

	KubeMgr.DeleteSecret("XYZ")
	KubeMgr.DeleteSecret("ABC")

	if !slices.Equal(publicKey1, publicKey2) {
		t.Error("publicKey not equal")
	}
	if !slices.Equal(privateKey1, privateKey2) {
		t.Error("privateKey not equal")
	}
}
