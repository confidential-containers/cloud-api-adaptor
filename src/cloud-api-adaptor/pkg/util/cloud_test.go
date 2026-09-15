package util

import (
	"testing"

	cri "github.com/containerd/containerd/pkg/cri/annotations"
	hypannotations "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/annotations"
	"github.com/stretchr/testify/assert"
)

func TestGetPodvmResourcesFromAnnotation(t *testing.T) {
	type args struct {
		annotations map[string]string
	}
	tests := []struct {
		name  string
		args  args
		want  int64
		want1 int64
		want2 int64
	}{
		// Add test cases with annotations for only vCPUs
		{
			name: "vCPUs only",
			args: args{
				annotations: map[string]string{
					hypannotations.DefaultVCPUs: "2",
				},
			},
			want:  2,
			want1: 0,
			want2: 0,
		},
		// Add test cases with annotations for only memory
		{
			name: "memory only",
			args: args{
				annotations: map[string]string{
					hypannotations.DefaultMemory: "2048",
				},
			},
			want:  0,
			want1: 2048,
			want2: 0,
		},
		// Add test cases with annotations for both vCPUs and memory
		{
			name: "vCPUs and memory",
			args: args{
				annotations: map[string]string{
					hypannotations.DefaultVCPUs:  "2",
					hypannotations.DefaultMemory: "2048",
				},
			},
			want:  2,
			want1: 2048,
			want2: 0,
		},
		// Add test cases with annotations for only GPU
		{
			name: "GPU only",
			args: args{
				annotations: map[string]string{
					hypannotations.DefaultGPUs: "1",
				},
			},
			want:  0,
			want1: 0,
			want2: 1,
		},
		// Add test cases with annotations for vCPUs, memory and GPU
		{
			name: "vCPUs, memory and GPU",
			args: args{
				annotations: map[string]string{
					hypannotations.DefaultVCPUs:  "2",
					hypannotations.DefaultMemory: "2048",
					hypannotations.DefaultGPUs:   "1",
				},
			},
			want:  2,
			want1: 2048,
			want2: 1,
		},

		// Add test cases with annotations with invalid values
		{
			name: "vCPUs and memory with invalid values",
			args: args{
				annotations: map[string]string{
					hypannotations.DefaultVCPUs:  "invalid",
					hypannotations.DefaultMemory: "invalid",
					hypannotations.DefaultGPUs:   "invalid",
				},
			},
			want:  0,
			want1: 0,
			want2: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2 := GetPodvmResourcesFromAnnotation(tt.args.annotations)
			if got != tt.want {
				t.Errorf("GetPodvmResourcesFromAnnotation() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetPodvmResourcesFromAnnotation() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("GetPodvmResourcesFromAnnotation() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}

func TestGetInstanceTypeFromAnnotation(t *testing.T) {
	type args struct {
		annotations map[string]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// Add test cases with annotations for only instance type
		{
			name: "instance type only",
			args: args{
				annotations: map[string]string{
					hypannotations.MachineType: "t2.small",
				},
			},
			want: "t2.small",
		},
		// Add test cases with annotations for only instance type with empty value
		{
			name: "instance type only with empty value",
			args: args{
				annotations: map[string]string{
					hypannotations.MachineType: "",
				},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetInstanceTypeFromAnnotation(tt.args.annotations); got != tt.want {
				t.Errorf("GetInstanceTypeFromAnnotation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetImageFromAnnotation(t *testing.T) {
	type args struct {
		annotations map[string]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// Add test cases with annotations for only image name
		{
			name: "image name only",
			args: args{
				annotations: map[string]string{
					hypannotations.ImagePath: "rhel9-os",
				},
			},
			want: "rhel9-os",
		},
		// Add test cases with annotations for only image name with empty value
		{
			name: "image name only with empty value",
			args: args{
				annotations: map[string]string{
					hypannotations.ImagePath: "",
				},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetImageFromAnnotation(tt.args.annotations); got != tt.want {
				t.Errorf("GetImageFromAnnotation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPodUID(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        string
	}{
		{
			name: "containerd sandbox uid",
			annotations: map[string]string{
				cri.SandboxUID:  "3f2b6c1e-8d4a-4f7b-9a2e-5c1d0e7f8a9b",
				cri.SandboxName: "my-pod",
			},
			want: "3f2b6c1e-8d4a-4f7b-9a2e-5c1d0e7f8a9b",
		},
		{
			name: "cri-o sandbox name passed to CreateVM",
			annotations: map[string]string{
				cri.SandboxName: "k8s_my-pod_default_3f2b6c1e-8d4a-4f7b-9a2e-5c1d0e7f8a9b_0",
			},
			want: "3f2b6c1e-8d4a-4f7b-9a2e-5c1d0e7f8a9b",
		},
		{
			name: "cri-o sandbox name on a container",
			annotations: map[string]string{
				"io.kubernetes.cri-o.SandboxName": "k8s_my-pod_default_3f2b6c1e-8d4a-4f7b-9a2e-5c1d0e7f8a9b_1",
			},
			want: "3f2b6c1e-8d4a-4f7b-9a2e-5c1d0e7f8a9b",
		},
		{
			name:        "containerd sandbox name without uid",
			annotations: map[string]string{cri.SandboxName: "my-pod"},
			want:        "",
		},
		{
			name:        "truncated cri-o sandbox name",
			annotations: map[string]string{cri.SandboxName: "k8s_my-pod_default"},
			want:        "",
		},
		{
			name:        "no annotations",
			annotations: map[string]string{},
			want:        "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GetPodUID(tt.annotations))
		})
	}
}
