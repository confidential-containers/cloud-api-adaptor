// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
)


const (
	IamServiceURL = "https://iam.cloud.ibm.com/identity/token"
	VpcServiceURL = "https://jp-tok.iaas.cloud.ibm.com/v1"
)

type VpcV1 interface {
        GetInstance(getInstanceOptions *vpcv1.GetInstanceOptions) (result *vpcv1.Instance, response *core.DetailedResponse, err error)
        CreateInstance(createInstanceOptions *vpcv1.CreateInstanceOptions) (result *vpcv1.Instance, response *core.DetailedResponse, err error)
        DeleteInstance(deleteInstanceOptions *vpcv1.DeleteInstanceOptions) (response *core.DetailedResponse, err error)
}


func NewVpcV1(apiKey string) (VpcV1, error) {
	vpcv1, err := vpcv1.NewVpcV1(&vpcv1.VpcV1Options{
		Authenticator: &core.IamAuthenticator{
			ApiKey: apiKey,
			URL:    IamServiceURL,
		},
		URL: VpcServiceURL,
	})
	if err != nil {
		return nil, err
	}
	return vpcv1, nil
}
