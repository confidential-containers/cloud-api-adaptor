package kubemgr

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/sshutil"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var logger = sshutil.Logger

var KubeMgr *KubeMgrStruct

const (
	cocoNamespace = "confidential-containers-system"
)

type KubeMgrStruct struct {
	Client        kubernetes.Interface //*kubernetes.Clientset
	CocoNamespace string
}

var SkipVerify bool

func getKubeConfigInVitro() (*rest.Config, error) {
	var kubeCfg *rest.Config
	var err error
	var flagFilePath string

	// Try to detect in-cluster config
	if kubeCfg, err = rest.InClusterConfig(); err == nil {
		return kubeCfg, nil
	}

	// Not running in cluster
	kubeConfigEnv := os.Getenv("KUBECONFIG")
	envVarFilePaths := filepath.SplitList(kubeConfigEnv)
	for _, envVarFilePath := range envVarFilePaths {
		if kubeCfg, err = clientcmd.BuildConfigFromFlags("", envVarFilePath); err == nil {
			return kubeCfg, nil
		}
	}

	if home := homedir.HomeDir(); home != "" {
		flagFilePath = *flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		flagFilePath = *flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	if kubeCfg, err = clientcmd.BuildConfigFromFlags("", flagFilePath); err == nil {
		return kubeCfg, nil
	}

	return nil, fmt.Errorf("no Config found to access KubeApi! Use KUBECONFIG or ~/.kube/config")
}

func InitKubeMgrInVitro() error {
	KubeMgr = &KubeMgrStruct{
		CocoNamespace: cocoNamespace,
	}

	kubeCfg, err := getKubeConfigInVitro()
	if err != nil {
		return err
	}

	// Create a secrets client
	if SkipVerify {
		kubeCfg.Insecure = true
		kubeCfg.CAData = nil
	}

	KubeMgr.Client, err = kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		return fmt.Errorf("failed to configure KubeApi using config: %w", err)
	}
	return nil
}

func InitKubeMgrInVivo() error {
	var err error
	KubeMgr = &KubeMgrStruct{
		CocoNamespace: cocoNamespace,
	}

	var kubeCfg *rest.Config

	// Try to detect in-cluster config
	if kubeCfg, err = rest.InClusterConfig(); err != nil {
		return fmt.Errorf("no config found to access KubeApi! err: %w", err)
	}

	KubeMgr.Client, err = kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		return fmt.Errorf("failed to configure KubeApi using config: %w", err)
	}
	return nil
}

func InitKubeMgrMock() {
	KubeMgr = &KubeMgrStruct{
		CocoNamespace: cocoNamespace,
		Client:        fake.NewSimpleClientset(),
	}
}

func (kubeMgr *KubeMgrStruct) ReadSecret(secretName string) (privateKey []byte, publicKey []byte, err error) {
	secrets := kubeMgr.Client.CoreV1().Secrets(kubeMgr.CocoNamespace)
	secret, err := secrets.Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		return
	}

	privateKey = secret.Data["privateKey"]
	publicKey = secret.Data["publicKey"]
	return
}

func (kubeMgr *KubeMgrStruct) DeleteSecret(secretName string) {
	secrets := kubeMgr.Client.CoreV1().Secrets(kubeMgr.CocoNamespace)
	if err := secrets.Delete(context.Background(), secretName, metav1.DeleteOptions{}); err != nil {
		logger.Printf("DeleteSecret '%s' error %v", secretName, err)
		return
	}
	logger.Printf("DeleteSecret '%s'", secretName)
}

func (kubeMgr *KubeMgrStruct) CreateSecret(secretName string) (privateKey []byte, publicKey []byte, err error) {
	bitSize := 4096
	clientPrivateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, nil, fmt.Errorf("CreateSecret rsa.GenerateKey err: %w", err)
	}

	// Validate Private Key
	err = clientPrivateKey.Validate()
	if err != nil {
		return nil, nil, fmt.Errorf("CreateSecret clientPrivateKey.Validate err: %w", err)
	}

	clientPublicKey, err := ssh.NewPublicKey(&clientPrivateKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("CreateSecret ssh.NewPublicKey err: %w", err)
	}

	publicKey = ssh.MarshalAuthorizedKey(clientPublicKey)

	privateKey = sshutil.RsaPrivateKeyPEM(clientPrivateKey)

	secrets := kubeMgr.Client.CoreV1().Secrets(kubeMgr.CocoNamespace)
	s := corev1.Secret{}
	s.Name = secretName
	s.Namespace = kubeMgr.CocoNamespace
	s.Data = map[string][]byte{}
	s.Data["privateKey"] = privateKey
	s.Data["publicKey"] = publicKey

	_, err = secrets.Create(context.Background(), &s, metav1.CreateOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("CreateSecret secrets.Create err: %w", err)
	}
	logger.Printf("CreateSecret '%s'", secretName)
	return
}
