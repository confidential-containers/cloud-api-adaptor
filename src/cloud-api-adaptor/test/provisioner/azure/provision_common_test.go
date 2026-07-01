// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"context"
	"testing"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	"github.com/stretchr/testify/require"
)

func TestAzureInstallChartConfigureCAAImage(t *testing.T) {
	origProps := AzureProps
	t.Cleanup(func() {
		AzureProps = origProps
	})

	newChart := func() *AzureInstallChart {
		return &AzureInstallChart{
			Helm: &pv.Helm{
				OverrideValues:          make(map[string]string),
				OverrideValueMap:        make(map[string]string),
				OverrideProviderValues:  make(map[string]string),
				OverrideProviderSecrets: make(map[string]string),
			},
		}
	}

	t.Run("name only", func(t *testing.T) {
		AzureProps = &AzureProperties{CaaImage: "quay.io/confidential-containers/cloud-api-adaptor"}
		chart := newChart()

		err := chart.Configure(context.Background(), nil, map[string]string{})
		require.NoError(t, err)
		require.Equal(t, "quay.io/confidential-containers/cloud-api-adaptor", chart.Helm.OverrideValues["image.name"])
		require.Empty(t, chart.Helm.OverrideValues["image.tag"])
	})

	t.Run("tag and digest", func(t *testing.T) {
		AzureProps = &AzureProperties{CaaImage: "quay.io/confidential-containers/cloud-api-adaptor:latest@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
		chart := newChart()

		err := chart.Configure(context.Background(), nil, map[string]string{})
		require.NoError(t, err)
		require.Equal(t, "quay.io/confidential-containers/cloud-api-adaptor", chart.Helm.OverrideValues["image.name"])
		require.Equal(t, "latest@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", chart.Helm.OverrideValues["image.tag"])
	})
}
