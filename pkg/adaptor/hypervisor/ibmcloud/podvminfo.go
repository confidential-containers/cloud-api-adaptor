//go:build ibmcloud

package ibmcloud

import (
	"context"
	"time"

	pb "github.com/confidential-containers/cloud-api-adaptor/proto/podvminfo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type podVMInfoService struct {
	srv *hypervisorService
}

func newPodVMInfoService(srv *hypervisorService) pb.PodVMInfoService {
	return &podVMInfoService{srv}
}

func (s *podVMInfoService) GetInfo(ctx context.Context, req *pb.GetInfoRequest) (*pb.GetInfoResponse, error) {

	s.srv.Lock()
	defer s.srv.Unlock()

	for {

		var vsi string

		for _, sandbox := range s.srv.sandboxes {
			if sandbox.namespace == req.PodNamespace && sandbox.pod == req.PodName {
				vsi = sandbox.vsi
				break
			}
		}

		if vsi != "" {
			return &pb.GetInfoResponse{VMID: vsi}, nil
		}

		if !req.Wait {
			break
		}
		s.srv.Unlock()
		time.Sleep(5 * time.Second) // TODO: use wait/notify here
		s.srv.Lock()
	}

	st := status.Error(codes.NotFound, "vsi was not found")

	return nil, st
}
