// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package wrapper

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type IdentityService struct {
	TargetEndpoint string
}

func NewIdentityService(targetEndpoint string) *IdentityService {
	return &IdentityService{
		TargetEndpoint: fmt.Sprintf("unix://%s", targetEndpoint),
	}
}

func (s *IdentityService) redirect(ctx context.Context, req interface{}, fn func(context.Context, csi.IdentityClient)) error {
	conn, err := grpc.Dial(s.TargetEndpoint, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := csi.NewIdentityClient(conn)

	fn(ctx, client)

	return nil
}

func (s *IdentityService) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (res *csi.GetPluginInfoResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.IdentityClient) {
		res, err = client.GetPluginInfo(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *IdentityService) Probe(ctx context.Context, req *csi.ProbeRequest) (res *csi.ProbeResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.IdentityClient) {
		res, err = client.Probe(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *IdentityService) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (res *csi.GetPluginCapabilitiesResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.IdentityClient) {
		res, err = client.GetPluginCapabilities(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}
