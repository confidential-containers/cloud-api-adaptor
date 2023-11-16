package provider

import (
	"fmt"
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

func TestSortInstanceTypesOnMemory(t *testing.T) {
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add benchmark
			start := time.Now()
			if got := SortInstanceTypesOnMemory(tt.args.instanceTypeSpecList); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SortInstanceTypesOnMemory() = %v, want %v", got, tt.want)
			}
			elapsed := time.Since(start)
			fmt.Printf("SortInstanceTypesOnMemory() took %s\n", elapsed)
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
