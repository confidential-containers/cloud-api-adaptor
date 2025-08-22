package test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/kubemgr"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/sshutil"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreatePKCS8Secret(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Errorf("createPKCS8Keys ed25519.GenerateKey err: %v", err)
	}

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Errorf("createPKCS8Keys MarshalPKCS8PrivateKey err: %v", err)
	}
	kbscPrivatePem := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privateKeyBytes,
		},
	)
	secrets := kubemgr.KubeMgr.Client.CoreV1().Secrets(kubemgr.KubeMgr.CocoNamespace)
	s := corev1.Secret{}
	s.Name = sshutil.KBSClientSecret
	s.Namespace = kubemgr.KubeMgr.CocoNamespace
	s.Data = map[string][]byte{}
	s.Data["privateKey"] = kbscPrivatePem

	_, err = secrets.Create(context.Background(), &s, metav1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}
}
