package provider

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestVerifyCloudInstanceType(t *testing.T) {
	type args struct {
		instanceType        string
		instanceTypes       []string
		defaultInstanceType string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// Add test case with instanceType="t2.small", instanceTypes=["t2.small, t2.medium"], defaultInstanceType="t2.medium"
		{
			name: "instanceType=t2.small, instanceTypes=[t2.small, t2.medium], defaultInstanceType=t2.medium",
			args: args{
				instanceType:        "t2.small",
				instanceTypes:       []string{"t2.small", "t2.medium"},
				defaultInstanceType: "t2.medium",
			},
			want:    "t2.small",
			wantErr: false,
		},
		// Add test case with instanceType="t2.small", instanceTypes=["t2.medium"], defaultInstanceType="t2.medium"
		{
			name: "instanceType=t2.small, instanceTypes=[t2.medium], defaultInstanceType=t2.medium",
			args: args{
				instanceType:        "t2.small",
				instanceTypes:       []string{"t2.medium"},
				defaultInstanceType: "t2.medium",
			},
			want:    "",
			wantErr: true,
		},
		// Add test case with instanceType="", instanceTypes=["t2.medium"], defaultInstanceType="t2.medium"
		{
			name: "instanceType=, instanceTypes=[t2.medium], defaultInstanceType=t2.medium",
			args: args{
				instanceType:        "",
				instanceTypes:       []string{"t2.medium"},
				defaultInstanceType: "t2.medium",
			},
			want:    "t2.medium",
			wantErr: false,
		},
		// Add test case with instanceType="", instanceTypes=[], defaultInstanceType="t2.medium"
		{
			name: "instanceType=, instanceTypes=[], defaultInstanceType=t2.medium",
			args: args{
				instanceType:        "",
				instanceTypes:       []string{},
				defaultInstanceType: "t2.medium",
			},
			want:    "t2.medium",
			wantErr: false,
		},
		// Add test case with instanceType="t2.small", instanceTypes=[], defaultInstanceType="t2.medium"
		{
			name: "instanceType=t2.small, instanceTypes=[], defaultInstanceType=t2.medium",
			args: args{
				instanceType:        "t2.small",
				instanceTypes:       []string{},
				defaultInstanceType: "t2.medium",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VerifyCloudInstanceType(tt.args.instanceType, tt.args.instanceTypes, tt.args.defaultInstanceType)

			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyCloudInstanceType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("VerifyCloudInstanceType() = %v, want %v", got, tt.want)
			}

		})
	}
}

func TestSortInstanceTypesOnResources(t *testing.T) {
	type args struct {
		instanceTypeSpecList []InstanceTypeSpec
	}
	tests := []struct {
		name string
		args args
		want []InstanceTypeSpec
	}{

		// Add test case with instanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}]
		{
			name: "instanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}]",
			args: args{
				instanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "t2.small",
						VCPUs:        2,
						Memory:       6,
					},
					{
						InstanceType: "t2.medium",
						VCPUs:        4,
						Memory:       8,
					},
					{
						InstanceType: "t2.large",
						VCPUs:        8,
						Memory:       16,
					},
				},
			},
			want: []InstanceTypeSpec{
				{
					InstanceType: "t2.small",
					VCPUs:        2,
					Memory:       6,
				},
				{
					InstanceType: "t2.medium",
					VCPUs:        4,
					Memory:       8,
				},
				{
					InstanceType: "t2.large",
					VCPUs:        8,
					Memory:       16,
				},
			},
		},
		// Add test case with instanceTypeSpecList=[{t2.small, 2, 6}, {t2.large, 8, 16}, {t2.medium, 4, 8}]
		{
			name: "instanceTypeSpecList=[{t2.small, 2, 6}, {t2.large, 8, 16}, {t2.medium, 4, 8}]",
			args: args{
				instanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "t2.small",
						VCPUs:        2,
						Memory:       6,
					},
					{
						InstanceType: "t2.large",
						VCPUs:        8,
						Memory:       16,
					},
					{
						InstanceType: "t2.medium",
						VCPUs:        4,
						Memory:       8,
					},
				},
			},
			want: []InstanceTypeSpec{
				{
					InstanceType: "t2.small",
					VCPUs:        2,
					Memory:       6,
				},
				{
					InstanceType: "t2.medium",
					VCPUs:        4,
					Memory:       8,
				},
				{
					InstanceType: "t2.large",
					VCPUs:        8,
					Memory:       16,
				},
			},
		},
		// Add test case with instanceTypeSpecList=[{t2.medium, 4, 8}, {t2.small, 2, 6}, {t2.large, 8, 16}]
		{
			name: "instanceTypeSpecList=[{t2.medium, 4, 8}, {t2.small, 2, 6}, {t2.large, 8, 16}]",
			args: args{
				instanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "t2.medium",
						VCPUs:        4,
						Memory:       8,
					},
					{
						InstanceType: "t2.small",
						VCPUs:        2,
						Memory:       6,
					},
					{
						InstanceType: "t2.large",
						VCPUs:        8,
						Memory:       16,
					},
				},
			},
			want: []InstanceTypeSpec{
				{
					InstanceType: "t2.small",
					VCPUs:        2,
					Memory:       6,
				},
				{
					InstanceType: "t2.medium",
					VCPUs:        4,
					Memory:       8,
				},
				{
					InstanceType: "t2.large",
					VCPUs:        8,
					Memory:       16,
				},
			},
		},

		// Add test case with instanceTypeSpecList=[{p2.medium, 4, 8, 2}, {p2.small, 2, 6, 1}, {p2.large, 8, 16, 4}]
		{
			name: "instanceTypeSpecList=[{p2.medium, 4, 8, 2}, {p2.small, 2, 6, 1}, {p2.large, 8, 16, 4}]",
			args: args{
				instanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "p2.medium",
						VCPUs:        4,
						Memory:       8,
						GPUs:         2,
					},
					{
						InstanceType: "p2.small",
						VCPUs:        2,
						Memory:       6,
						GPUs:         1,
					},
					{
						InstanceType: "p2.large",
						VCPUs:        8,
						Memory:       16,
						GPUs:         4,
					},
				},
			},
			want: []InstanceTypeSpec{
				{
					InstanceType: "p2.small",
					VCPUs:        2,
					Memory:       6,
					GPUs:         1,
				},
				{
					InstanceType: "p2.medium",
					VCPUs:        4,
					Memory:       8,
					GPUs:         2,
				},
				{
					InstanceType: "p2.large",
					VCPUs:        8,
					Memory:       16,
					GPUs:         4,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add benchmark
			start := time.Now()
			if got := SortInstanceTypesOnResources(tt.args.instanceTypeSpecList); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SortInstanceTypesOnResources() = %v, want %v", got, tt.want)
			}
			elapsed := time.Since(start)
			fmt.Printf("SortInstanceTypesOnResources() took %s\n", elapsed)
		})
	}
}

func TestGetBestFitInstanceType(t *testing.T) {
	type args struct {
		sortedInstanceTypeSpecList []InstanceTypeSpec
		vcpus                      int64
		memory                     int64
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// Add test case with sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=2, memory=6
		{
			name: "sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=2, memory=6",
			args: args{
				sortedInstanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "t2.small",
						VCPUs:        2,
						Memory:       6,
					},
					{
						InstanceType: "t2.medium",
						VCPUs:        4,
						Memory:       8,
					},
					{
						InstanceType: "t2.large",
						VCPUs:        8,
						Memory:       16,
					},
				},
				vcpus:  2,
				memory: 6,
			},
			want:    "t2.small",
			wantErr: false,
		},
		// Add test case with sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=4, memory=8
		{
			name: "sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=4, memory=8",
			args: args{
				sortedInstanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "t2.small",
						VCPUs:        2,
						Memory:       6,
					},
					{
						InstanceType: "t2.medium",
						VCPUs:        4,
						Memory:       8,
					},
					{
						InstanceType: "t2.large",
						VCPUs:        8,
						Memory:       16,
					},
				},
				vcpus:  4,
				memory: 8,
			},
			want:    "t2.medium",
			wantErr: false,
		},
		// Add test case with sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=4, memory=16
		{
			name: "sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=4, memory=16",
			args: args{
				sortedInstanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "t2.small",
						VCPUs:        2,
						Memory:       6,
					},
					{
						InstanceType: "t2.medium",
						VCPUs:        4,
						Memory:       8,
					},
					{
						InstanceType: "t2.large",
						VCPUs:        8,
						Memory:       16,
					},
				},
				vcpus:  4,
				memory: 16,
			},
			want:    "t2.large",
			wantErr: false,
		},
		// Add test case with sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {p2.medium, 4, 8, 2}, {t2.large, 8, 16}], vcpus=4, memory=8
		{
			name: "sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {p2.medium, 4, 8, 2}, {t2.large, 8, 16}], vcpus=4, memory=8",
			args: args{
				sortedInstanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "t2.small",
						VCPUs:        2,
						Memory:       6,
					},
					{
						InstanceType: "p2.medium",
						VCPUs:        4,
						Memory:       8,
						GPUs:         2,
					},
					{
						InstanceType: "t2.large",
						VCPUs:        8,
						Memory:       16,
					},
				},
				vcpus:  4,
				memory: 8,
			},
			want:    "t2.large",
			wantErr: false,
		},

		// Add test case with sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=4, memory=32
		{
			name: "sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=4, memory=32",
			args: args{
				sortedInstanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "t2.small",
						VCPUs:        2,
						Memory:       6,
					},
					{
						InstanceType: "t2.medium",
						VCPUs:        4,
						Memory:       8,
					},
					{
						InstanceType: "t2.large",
						VCPUs:        8,
						Memory:       16,
					},
				},
				vcpus:  4,
				memory: 32,
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add benchmark
			start := time.Now()
			got, err := GetBestFitInstanceType(tt.args.sortedInstanceTypeSpecList, tt.args.vcpus, tt.args.memory)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBestFitInstanceType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			elapsed := time.Since(start)
			fmt.Printf("GetBestFitInstanceType() took %s\n", elapsed)
			if got != tt.want {
				t.Errorf("GetBestFitInstanceType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVerifySSHKeyFile(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() (string, error)
		expectedErr bool
	}{
		{
			name: "File does not exist",
			setup: func() (string, error) {
				return "/non/existent/file", nil
			},
			expectedErr: true,
		},
		{
			name: "File with incorrect permissions",
			setup: func() (string, error) {
				file, err := os.CreateTemp("", "sshkey")
				if err != nil {
					return "", err
				}
				defer file.Close()
				if err := os.Chmod(file.Name(), 0644); err != nil {
					return "", err
				}
				return file.Name(), nil
			},
			expectedErr: true,
		},
		{
			name: "File with invalid SSH key content",
			setup: func() (string, error) {
				file, err := os.CreateTemp("", "sshkey")
				if err != nil {
					return "", err
				}
				defer file.Close()
				if _, err := file.WriteString("invalid-key-content"); err != nil {
					return "", err
				}
				if err := os.Chmod(file.Name(), 0600); err != nil {
					return "", err
				}
				return file.Name(), nil
			},
			expectedErr: true,
		},
		{
			name: "File with valid SSH key content",
			setup: func() (string, error) {
				file, err := os.CreateTemp("", "sshkey")
				if err != nil {
					return "", err
				}
				defer file.Close()
				if _, err := file.WriteString("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAYgc9x91raNF1kh/7+XA9EpN4IoQnWC5kv1g107wVmt"); err != nil {
					return "", err
				}
				if err := os.Chmod(file.Name(), 0600); err != nil {
					return "", err
				}
				return file.Name(), nil

			},
			expectedErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sshKeyFile, err := tt.setup()
			if err != nil {
				t.Errorf("Error setting up test: %v", err)
				return
			}
			// Display the file content
			fmt.Printf("File content: %s\n", sshKeyFile)
			err = VerifySSHKeyFile(sshKeyFile)
			if (err != nil) != tt.expectedErr {
				t.Errorf("VerifySSHKeyFile() error = %v, expectedErr %v", err, tt.expectedErr)
			}
		})
	}
}

func TestGetBestFitInstanceTypeWithGPU(t *testing.T) {
	tests := []struct {
		name          string
		specList      []InstanceTypeSpec
		gpus          int64
		vcpus         int64
		memory        int64
		expected      string
		expectedError bool
	}{
		{
			name: "exact match",
			specList: []InstanceTypeSpec{
				{InstanceType: "small-gpu", GPUs: 1, VCPUs: 2, Memory: 4096},
				{InstanceType: "medium-gpu", GPUs: 2, VCPUs: 4, Memory: 8192},
			},
			gpus:          1,
			vcpus:         2,
			memory:        4096,
			expected:      "small-gpu",
			expectedError: false,
		},
		{
			name: "next best fit",
			specList: []InstanceTypeSpec{
				{InstanceType: "small-gpu", GPUs: 1, VCPUs: 2, Memory: 4096},
				{InstanceType: "medium-gpu", GPUs: 2, VCPUs: 4, Memory: 8192},
			},
			gpus:          1,
			vcpus:         3,
			memory:        6144,
			expected:      "medium-gpu",
			expectedError: false,
		},
		{
			name: "no match found",
			specList: []InstanceTypeSpec{
				{InstanceType: "small-gpu", GPUs: 1, VCPUs: 2, Memory: 4096},
				{InstanceType: "medium-gpu", GPUs: 2, VCPUs: 4, Memory: 8192},
			},
			gpus:          4,
			vcpus:         8,
			memory:        16384,
			expected:      "",
			expectedError: true,
		},
		{
			name:          "empty spec list",
			specList:      []InstanceTypeSpec{},
			gpus:          1,
			vcpus:         2,
			memory:        4096,
			expected:      "",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Sort the spec list first as required by the function
			sortedList := SortInstanceTypesOnResources(tt.specList)

			result, err := GetBestFitInstanceTypeWithGPU(sortedList, tt.gpus, tt.vcpus, tt.memory)

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %s but got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestSelectInstanceTypeToUse(t *testing.T) {
	type args struct {
		spec                InstanceTypeSpec
		specList            []InstanceTypeSpec
		validInstanceTypes  []string
		defaultInstanceType string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "spec.InstanceType takes priority over GPU requirements",
			args: args{
				spec: InstanceTypeSpec{
					InstanceType: "m5.large",
					GPUs:         2,
					VCPUs:        4,
					Memory:       8,
				},
				specList: []InstanceTypeSpec{
					{InstanceType: "p3.xlarge", GPUs: 1, VCPUs: 4, Memory: 16},
					{InstanceType: "p3.2xlarge", GPUs: 2, VCPUs: 8, Memory: 32},
					{InstanceType: "m5.large", GPUs: 0, VCPUs: 2, Memory: 8},
				},
				validInstanceTypes:  []string{"m5.large", "p3.xlarge", "p3.2xlarge"},
				defaultInstanceType: "t3.medium",
			},
			want:    "m5.large",
			wantErr: false,
		},
		{
			name: "spec.InstanceType takes priority over vCPU/memory requirements",
			args: args{
				spec: InstanceTypeSpec{
					InstanceType: "t3.micro",
					GPUs:         0,
					VCPUs:        4,
					Memory:       16,
				},
				specList: []InstanceTypeSpec{
					{InstanceType: "t3.micro", GPUs: 0, VCPUs: 1, Memory: 1},
					{InstanceType: "t3.large", GPUs: 0, VCPUs: 2, Memory: 8},
					{InstanceType: "t3.xlarge", GPUs: 0, VCPUs: 4, Memory: 16},
				},
				validInstanceTypes:  []string{"t3.micro", "t3.large", "t3.xlarge"},
				defaultInstanceType: "t3.medium",
			},
			want:    "t3.micro",
			wantErr: false,
		},
		{
			name: "fallback to GPU requirements when spec.InstanceType is empty",
			args: args{
				spec: InstanceTypeSpec{
					InstanceType: "",
					GPUs:         1,
					VCPUs:        4,
					Memory:       16,
				},
				specList: []InstanceTypeSpec{
					{InstanceType: "p3.xlarge", GPUs: 1, VCPUs: 4, Memory: 16},
					{InstanceType: "p3.2xlarge", GPUs: 2, VCPUs: 8, Memory: 32},
					{InstanceType: "m5.large", GPUs: 0, VCPUs: 2, Memory: 8},
				},
				validInstanceTypes:  []string{"m5.large", "p3.xlarge", "p3.2xlarge"},
				defaultInstanceType: "t3.medium",
			},
			want:    "p3.xlarge",
			wantErr: false,
		},
		{
			name: "fallback to vCPU/memory when spec.InstanceType empty and no GPU",
			args: args{
				spec: InstanceTypeSpec{
					InstanceType: "",
					GPUs:         0,
					VCPUs:        4,
					Memory:       16,
				},
				specList: []InstanceTypeSpec{
					{InstanceType: "t3.large", GPUs: 0, VCPUs: 2, Memory: 8},
					{InstanceType: "t3.xlarge", GPUs: 0, VCPUs: 4, Memory: 16},
					{InstanceType: "t3.2xlarge", GPUs: 0, VCPUs: 8, Memory: 32},
				},
				validInstanceTypes:  []string{"t3.large", "t3.xlarge", "t3.2xlarge"},
				defaultInstanceType: "t3.medium",
			},
			want:    "t3.xlarge",
			wantErr: false,
		},
		{
			name: "spec.InstanceType not in validInstanceTypes should fail verification",
			args: args{
				spec: InstanceTypeSpec{
					InstanceType: "invalid.type",
					GPUs:         0,
					VCPUs:        2,
					Memory:       4,
				},
				specList: []InstanceTypeSpec{
					{InstanceType: "t3.small", GPUs: 0, VCPUs: 2, Memory: 2},
				},
				validInstanceTypes:  []string{"t3.small"},
				defaultInstanceType: "t3.small",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "use default when no specs provided",
			args: args{
				spec: InstanceTypeSpec{
					InstanceType: "",
					GPUs:         0,
					VCPUs:        0,
					Memory:       0,
				},
				specList:            []InstanceTypeSpec{},
				validInstanceTypes:  []string{},
				defaultInstanceType: "t3.medium",
			},
			want:    "t3.medium",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Sort the spec list first as required by the function
			sortedList := SortInstanceTypesOnResources(tt.args.specList)

			got, err := SelectInstanceTypeToUse(tt.args.spec, sortedList, tt.args.validInstanceTypes, tt.args.defaultInstanceType)
			if (err != nil) != tt.wantErr {
				t.Errorf("SelectInstanceTypeToUse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SelectInstanceTypeToUse() = %v, want %v", got, tt.want)
			}
		})
	}
}
