package util

import (
	"testing"

	hypannotations "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/annotations"
)

func TestGetCPUAndMemoryFromAnnotation(t *testing.T) {
	type args struct {
		annotations map[string]string
	}
	tests := []struct {
		name  string
		args  args
		want  int64
		want1 int64
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
		},
		// Add test cases with annotations for vCPUs and memory with invalid values
		{
			name: "vCPUs and memory with invalid values",
			args: args{
				annotations: map[string]string{
					hypannotations.DefaultVCPUs:  "invalid",
					hypannotations.DefaultMemory: "invalid",
				},
			},
			want:  0,
			want1: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetCPUAndMemoryFromAnnotation(tt.args.annotations)
			if got != tt.want {
				t.Errorf("GetCPUAndMemoryFromAnnotation() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetCPUAndMemoryFromAnnotation() got1 = %v, want %v", got1, tt.want1)
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
