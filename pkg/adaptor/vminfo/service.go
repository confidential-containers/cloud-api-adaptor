package vminfo

import (
	"context"
	"errors"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	pb "github.com/confidential-containers/cloud-api-adaptor/proto/podvminfo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type podVMInfoService struct {
	cloudService cloud.Service
}

func NewService(cloudService cloud.Service) pb.PodVMInfoService {
	return &podVMInfoService{cloudService}
}

func (s *podVMInfoService) GetInfo(ctx context.Context, req *pb.GetInfoRequest) (*pb.GetInfoResponse, error) {

	instanceID, err := s.cloudService.GetInstanceID(ctx, req.PodNamespace, req.PodName, req.Wait)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, status.Errorf(codes.DeadlineExceeded, "getting VM ID for %s:%s: %s", req.PodNamespace, req.PodName, err.Error())
		} else {
			return nil, status.Errorf(codes.Unknown, "getting VM ID for %s:%s: %s", req.PodNamespace, req.PodName, err.Error())
		}
	}

	if instanceID != "" {
		return &pb.GetInfoResponse{VMID: instanceID}, nil
	}

	return nil, status.Errorf(codes.NotFound, "VM ID for %s:%s was not found", req.PodNamespace, req.PodName)
}
