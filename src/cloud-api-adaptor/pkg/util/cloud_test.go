package util

import (
	"testing"

	hypannotations "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/annotations"
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
			r := GetPodVMResourcesFromAnnotation(tt.args.annotations)
			got, got1, got2 := r.VCPUs, r.Memory, r.GPUs
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
