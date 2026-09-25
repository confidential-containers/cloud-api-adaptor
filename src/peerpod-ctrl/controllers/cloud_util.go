/*
Copyright Confidential Containers Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"flag"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

const (
	ppConfigMap = "peer-pods-cm"
	ppSecret    = "peer-pods-secret"
)

func loadCloudConfigs(ctx context.Context, reader client.Reader, namespace string) error {
	if namespace == "" {
		return fmt.Errorf("PEERPODS_NAMESPACE is not set")
	}

	cm := corev1.ConfigMap{}
	secret := corev1.Secret{}

	var cmErr error
	if cmErr = reader.Get(ctx, types.NamespacedName{Name: ppConfigMap, Namespace: namespace}, &cm); cmErr == nil {
		for k, v := range cm.Data {
			os.Setenv(k, v)
		}
	}

	var secretErr error
	if secretErr = reader.Get(ctx, types.NamespacedName{Name: ppSecret, Namespace: namespace}, &secret); secretErr == nil {
		for k, v := range secret.Data {
			os.Setenv(k, string(v))
		}
	}

	if cm.Data == nil && secret.Data == nil {
		return fmt.Errorf("ConfigMap Error: %v, Secret Error: %v", cmErr, secretErr)
	}

	return nil
}

// GetProvider loads a cloud provider by name. loadCloudConfigs must be called
// first to populate environment variables that ParseCmd reads.
func GetProvider(cloudName string) (provider.Provider, error) {
	if cloud := provider.Get(cloudName); cloud != nil {
		dummyFlags := flag.NewFlagSet(cloudName, flag.ContinueOnError)
		cloud.ParseCmd(dummyFlags)

		p, err := cloud.NewProvider()
		if err != nil {
			return nil, err
		}
		return p, nil
	}

	return nil, fmt.Errorf("%s cloud provider not supported", cloudName)
}
