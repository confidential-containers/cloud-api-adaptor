// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package alibabacloud

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	ecs "github.com/alibabacloud-go/ecs-20140526/v4/client"
	"github.com/alibabacloud-go/tea/tea"
	vpc "github.com/alibabacloud-go/vpc-20160428/v6/client"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
	kconf "sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var AlibabaCloudProps = &AlibabaCloudProvisioner{}

// Vpc holds Alibaba Cloud VPC networking resources used by e2e.
type Vpc struct {
	BaseName        string
	CidrBlock       string
	ID              string
	Region          string
	SecurityGroupID string
	VSwitchID       string
	ZoneID          string
	// IngressCIDRs limits security-group ingress sources (e.g. runner /32).
	// Empty means detect the host public IPv4 at CreateVPC time.
	IngressCIDRs []string
}

// CustomImage represents an imported ECS custom image.
type CustomImage struct {
	BaseName string
	ID       string
	Name     string
}

// OSSBucket represents an OSS bucket used for image import.
type OSSBucket struct {
	Name   string
	Object string
	Region string
}

// Cluster defines create/delete/access interfaces to Kubernetes clusters.
type Cluster interface {
	CreateCluster() error
	DeleteCluster() error
	GetKubeconfigFile() (string, error)
}

// OnPremCluster represents an existing and running cluster.
type OnPremCluster struct{}

// AlibabaCloudProvisioner implements the CloudProvisioner interface.
type AlibabaCloudProvisioner struct {
	AccessKeyID        string
	AccessKeySecret    string
	SecurityToken      string
	ecsClient          *ecs.Client
	vpcClient          *vpc.Client
	containerRuntime   string
	Cluster            Cluster
	CaaImage           string
	Disablecvm         string
	PauseImage         string
	Image              *CustomImage
	Bucket             *OSSBucket
	Vpc                *Vpc
	PublicIP           string
	TunnelType         string
	VxlanPort          string
	SSHKpName          string
	PodvmInstanceType  string
	PeerpodsSecretName string
	Region             string
}

// AlibabaCloudInstallChart implements the InstallChart interface.
type AlibabaCloudInstallChart struct {
	Helm *pv.Helm
}

// NewAlibabaCloudProvisioner instantiates the Alibaba Cloud provisioner.
func NewAlibabaCloudProvisioner(properties map[string]string) (pv.CloudProvisioner, error) {
	accessKeyID, accessKeySecret, securityToken := credentialsFromEnv()
	if accessKeyID == "" || accessKeySecret == "" {
		return nil, fmt.Errorf("Alibaba Cloud credentials not found in environment (ALIBABACLOUD_ACCESS_KEY_ID/SECRET or ALIBABA_CLOUD_ACCESS_KEY_ID/SECRET)")
	}

	region := properties["region"]
	if region == "" {
		region = "cn-beijing"
	}

	if properties["resources_basename"] == "" {
		properties["resources_basename"] = "caa-e2e-test-" + strconv.FormatInt(time.Now().Unix(), 10)
	}

	cfg := &openapi.Config{
		AccessKeyId:     tea.String(accessKeyID),
		AccessKeySecret: tea.String(accessKeySecret),
		SecurityToken:   tea.String(securityToken),
		RegionId:        tea.String(region),
	}

	ecsClient, err := ecs.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create ECS client: %w", err)
	}
	vpcClient, err := vpc.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create VPC client: %w", err)
	}

	var cluster Cluster
	switch properties["cluster_type"] {
	case "", "onprem":
		cluster = NewOnPremCluster()
		properties["use_public_ip"] = "true"
	default:
		return nil, fmt.Errorf("cluster type '%s' not implemented", properties["cluster_type"])
	}

	podvmInstanceType := properties["podvm_instance_type"]
	if podvmInstanceType == "" {
		podvmInstanceType = "ecs.g8i.xlarge"
	}

	cidrBlock := properties["alibabacloud_vpc_cidrblock"]
	if cidrBlock == "" {
		cidrBlock = "10.0.0.0/24"
	}

	ingressCIDRs, err := parseIngressCIDRs(properties["sg_ingress_cidrs"])
	if err != nil {
		return nil, err
	}

	AlibabaCloudProps = &AlibabaCloudProvisioner{
		AccessKeyID:      accessKeyID,
		AccessKeySecret:  accessKeySecret,
		SecurityToken:    securityToken,
		ecsClient:        ecsClient,
		vpcClient:        vpcClient,
		containerRuntime: properties["container_runtime"],
		Cluster:          cluster,
		CaaImage:         properties["CAA_IMAGE"],
		Disablecvm:       properties["disablecvm"],
		PauseImage:       properties["pause_image"],
		Image: &CustomImage{
			BaseName: properties["resources_basename"],
			ID:       properties["podvm_image_id"],
		},
		Bucket: &OSSBucket{
			Name:   sanitizeOSSBucketName(properties["resources_basename"] + "-bucket"),
			Region: region,
		},
		Vpc: &Vpc{
			BaseName:        properties["resources_basename"],
			CidrBlock:       cidrBlock,
			ID:              properties["alibabacloud_vpc_id"],
			Region:          region,
			SecurityGroupID: properties["alibabacloud_vpc_sg_id"],
			VSwitchID:       properties["alibabacloud_vpc_vswitch_id"],
			ZoneID:          properties["alibabacloud_zone_id"],
			IngressCIDRs:    ingressCIDRs,
		},
		PublicIP:           properties["use_public_ip"],
		TunnelType:         properties["tunnel_type"],
		VxlanPort:          properties["vxlan_port"],
		SSHKpName:          properties["ssh_kp_name"],
		PodvmInstanceType:  podvmInstanceType,
		PeerpodsSecretName: properties["peerpods_secret_name"],
		Region:             region,
	}

	return AlibabaCloudProps, nil
}

func (a *AlibabaCloudProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	if err := a.Cluster.CreateCluster(); err != nil {
		return err
	}
	kubeconfigPath, err := a.Cluster.GetKubeconfigFile()
	if err != nil {
		return err
	}
	*cfg = *envconf.NewWithKubeConfig(kubeconfigPath)
	return nil
}

func (a *AlibabaCloudProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	if a.Vpc.ID == "" {
		log.Infof("Create Alibaba Cloud VPC in region %s", a.Region)
		if err := a.Vpc.createVpc(a.vpcClient); err != nil {
			return err
		}
		log.Infof("VPC Id: %s", a.Vpc.ID)
	}

	if a.Vpc.VSwitchID == "" {
		log.Infof("Create vSwitch on VPC %s", a.Vpc.ID)
		if err := a.Vpc.createVSwitch(a.vpcClient, a.ecsClient, a.PodvmInstanceType); err != nil {
			return err
		}
		log.Infof("vSwitch Id: %s", a.Vpc.VSwitchID)
	}

	if a.Vpc.SecurityGroupID == "" {
		if len(a.Vpc.IngressCIDRs) == 0 {
			ip, err := detectPublicIPv4()
			if err != nil {
				return fmt.Errorf("detect runner public IP for security group ingress: %w (set sg_ingress_cidrs to override)", err)
			}
			a.Vpc.IngressCIDRs = []string{ip + "/32"}
			log.Infof("Using detected public IP for SG ingress: %s", a.Vpc.IngressCIDRs[0])
		}
		log.Infof("Create security group on VPC %s (ingress from %s)", a.Vpc.ID, strings.Join(a.Vpc.IngressCIDRs, ","))
		if err := a.Vpc.setupSecurityGroup(a.ecsClient); err != nil {
			return err
		}
		log.Infof("Security group Id: %s", a.Vpc.SecurityGroupID)
	}

	return nil
}

func (a *AlibabaCloudProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	return a.Cluster.DeleteCluster()
}

func (a *AlibabaCloudProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	vpcRes := a.Vpc

	if vpcRes.VSwitchID != "" {
		log.Infof("Delete instances and vSwitch: %s", vpcRes.VSwitchID)
		if err := vpcRes.deleteVSwitch(a.ecsClient, a.vpcClient); err != nil {
			return err
		}
	}

	if vpcRes.SecurityGroupID != "" {
		log.Infof("Delete security group: %s", vpcRes.SecurityGroupID)
		if err := vpcRes.deleteSecurityGroup(a.ecsClient); err != nil {
			return err
		}
	}

	if vpcRes.ID != "" {
		log.Infof("Delete VPC: %s", vpcRes.ID)
		if err := vpcRes.deleteVpc(a.vpcClient, a.ecsClient); err != nil {
			return err
		}
	}

	if a.Image.ID != "" {
		log.Infof("Delete custom image: %s", a.Image.ID)
		if _, err := a.ecsClient.DeleteImage(&ecs.DeleteImageRequest{
			RegionId: tea.String(a.Region),
			ImageId:  tea.String(a.Image.ID),
			Force:    tea.Bool(true),
		}); err != nil {
			log.Errorf("Failed to delete image: %v", err)
		}
	}

	if a.Bucket.Object != "" || a.Bucket.Name != "" {
		if err := a.deleteOSSBucket(); err != nil {
			log.Errorf("Failed to delete OSS bucket: %v", err)
		}
	}

	return nil
}

func (a *AlibabaCloudProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	return map[string]string{
		"CAA_IMAGE":            a.CaaImage,
		"CONTAINER_RUNTIME":    a.containerRuntime,
		"disablecvm":           a.Disablecvm,
		"pause_image":          a.PauseImage,
		"podvm_image_id":       a.Image.ID,
		"podvm_instance_type":  a.PodvmInstanceType,
		"security_group_ids":   a.Vpc.SecurityGroupID,
		"vswitch_id":           a.Vpc.VSwitchID,
		"ssh_kp_name":          a.SSHKpName,
		"region":               a.Region,
		"resources_basename":   a.Vpc.BaseName,
		"access_key_id":        a.AccessKeyID,
		"secret_access_key":    a.AccessKeySecret,
		"security_token":       a.SecurityToken,
		"use_public_ip":        a.PublicIP,
		"tunnel_type":          a.TunnelType,
		"peerpods_secret_name": a.PeerpodsSecretName,
		"vxlan_port":           a.VxlanPort,
	}
}

func (a *AlibabaCloudProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	log.Infof("Create OSS bucket '%s' in region %s", a.Bucket.Name, a.Bucket.Region)
	if err := a.createOSSBucket(); err != nil {
		return err
	}

	log.Infof("Upload image %s to OSS bucket '%s'", imagePath, a.Bucket.Name)
	if err := a.uploadImageToOSS(imagePath); err != nil {
		return err
	}

	imageNameSuffix := "-" + strconv.FormatInt(time.Now().Unix(), 10)
	imageName := strings.Replace(filepath.Base(imagePath), ".qcow2", imageNameSuffix, 1)
	// Image names must start with a letter.
	if imageName == "" || (imageName[0] >= '0' && imageName[0] <= '9') {
		imageName = "podvm-" + imageName
	}
	a.Image.Name = imageName

	log.Infof("Import ECS image with name: %s", imageName)
	if err := a.importImage(); err != nil {
		return err
	}
	log.Infof("New Image ID: %s", a.Image.ID)
	return nil
}

func (a *AlibabaCloudProvisioner) ossClient() (*oss.Client, error) {
	endpoint := fmt.Sprintf("https://oss-%s.aliyuncs.com", a.Region)
	opts := []oss.ClientOption{}
	if a.SecurityToken != "" {
		opts = append(opts, oss.SecurityToken(a.SecurityToken))
	}
	return oss.New(endpoint, a.AccessKeyID, a.AccessKeySecret, opts...)
}

func (a *AlibabaCloudProvisioner) createOSSBucket() error {
	client, err := a.ossClient()
	if err != nil {
		return err
	}
	exists, err := client.IsBucketExist(a.Bucket.Name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return client.CreateBucket(a.Bucket.Name, oss.StorageClass(oss.StorageStandard), oss.ACL(oss.ACLPrivate))
}

func (a *AlibabaCloudProvisioner) uploadImageToOSS(imagePath string) error {
	client, err := a.ossClient()
	if err != nil {
		return err
	}
	bucket, err := client.Bucket(a.Bucket.Name)
	if err != nil {
		return err
	}
	objectKey := filepath.Base(imagePath)
	if err := bucket.PutObjectFromFile(objectKey, imagePath); err != nil {
		return err
	}
	a.Bucket.Object = objectKey
	return nil
}

func (a *AlibabaCloudProvisioner) deleteOSSBucket() error {
	client, err := a.ossClient()
	if err != nil {
		return err
	}
	exists, err := client.IsBucketExist(a.Bucket.Name)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	bucket, err := client.Bucket(a.Bucket.Name)
	if err != nil {
		return err
	}
	marker := oss.Marker("")
	for {
		lor, err := bucket.ListObjects(marker)
		if err != nil {
			return err
		}
		for _, object := range lor.Objects {
			_ = bucket.DeleteObject(object.Key)
		}
		if !lor.IsTruncated {
			break
		}
		marker = oss.Marker(lor.NextMarker)
	}
	log.Infof("Delete OSS bucket: %s", a.Bucket.Name)
	return client.DeleteBucket(a.Bucket.Name)
}

func (a *AlibabaCloudProvisioner) importImage() error {
	req := &ecs.ImportImageRequest{
		RegionId:     tea.String(a.Region),
		ImageName:    tea.String(a.Image.Name),
		Description:  tea.String("Peer Pod VM image"),
		Architecture: tea.String("x86_64"),
		OSType:       tea.String("linux"),
		Platform:     tea.String("Ubuntu"),
		BootMode:     tea.String("UEFI"),
		DiskDeviceMapping: []*ecs.ImportImageRequestDiskDeviceMapping{
			{
				OSSBucket: tea.String(a.Bucket.Name),
				OSSObject: tea.String(a.Bucket.Object),
				Format:    tea.String("QCOW2"),
			},
		},
		Features: &ecs.ImportImageRequestFeatures{
			NvmeSupport: tea.String("supported"),
		},
		Tag: imageNameTags(a.Image.BaseName + "-img"),
	}

	result, err := a.ecsClient.ImportImage(req)
	if err != nil {
		return err
	}
	if result.Body == nil || result.Body.ImageId == nil {
		return fmt.Errorf("ImportImage returned empty ImageId")
	}
	a.Image.ID = tea.StringValue(result.Body.ImageId)

	deadline := time.Now().Add(30 * time.Minute)
	for time.Now().Before(deadline) {
		desc, err := a.ecsClient.DescribeImages(&ecs.DescribeImagesRequest{
			RegionId: tea.String(a.Region),
			ImageId:  tea.String(a.Image.ID),
		})
		if err != nil {
			return err
		}
		if desc.Body != nil && desc.Body.Images != nil && len(desc.Body.Images.Image) > 0 {
			status := tea.StringValue(desc.Body.Images.Image[0].Status)
			log.Infof("Image %s status: %s", a.Image.ID, status)
			if status == "Available" {
				return nil
			}
			if status == "CreateFailed" {
				return fmt.Errorf("image import failed for %s", a.Image.ID)
			}
		}
		time.Sleep(20 * time.Second)
	}
	return fmt.Errorf("timeout waiting for image %s to become Available", a.Image.ID)
}

func (v *Vpc) createVpc(client *vpc.Client) error {
	result, err := client.CreateVpc(&vpc.CreateVpcRequest{
		RegionId:  tea.String(v.Region),
		CidrBlock: tea.String(v.CidrBlock),
		VpcName:   tea.String(v.BaseName + "-vpc"),
		Tag:       vpcNameTags(v.BaseName + "-vpc"),
	})
	if err != nil {
		return err
	}
	v.ID = tea.StringValue(result.Body.VpcId)

	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		desc, err := client.DescribeVpcAttribute(&vpc.DescribeVpcAttributeRequest{
			VpcId:    tea.String(v.ID),
			RegionId: tea.String(v.Region),
		})
		if err != nil {
			return err
		}
		if tea.StringValue(desc.Body.Status) == "Available" {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for VPC %s to become Available", v.ID)
}

func (v *Vpc) selectZone(ecsClient *ecs.Client, instanceType string) (string, error) {
	if v.ZoneID != "" {
		return v.ZoneID, nil
	}

	result, err := ecsClient.DescribeAvailableResource(&ecs.DescribeAvailableResourceRequest{
		RegionId:            tea.String(v.Region),
		DestinationResource: tea.String("InstanceType"),
		InstanceType:        tea.String(instanceType),
	})
	if err != nil {
		return "", err
	}

	if result.Body != nil && result.Body.AvailableZones != nil {
		for _, zone := range result.Body.AvailableZones.AvailableZone {
			if tea.StringValue(zone.Status) == "Available" && zone.ZoneId != nil {
				return tea.StringValue(zone.ZoneId), nil
			}
		}
	}

	// Fallback: any zone in the region.
	zones, err := ecsClient.DescribeZones(&ecs.DescribeZonesRequest{
		RegionId: tea.String(v.Region),
	})
	if err != nil {
		return "", err
	}
	if zones.Body != nil && len(zones.Body.Zones.Zone) > 0 {
		return tea.StringValue(zones.Body.Zones.Zone[0].ZoneId), nil
	}
	return "", fmt.Errorf("no available zone found in region %s for instance type %s", v.Region, instanceType)
}

func (v *Vpc) createVSwitch(vpcClient *vpc.Client, ecsClient *ecs.Client, instanceType string) error {
	zoneID, err := v.selectZone(ecsClient, instanceType)
	if err != nil {
		return err
	}
	v.ZoneID = zoneID

	// Use a subset of the VPC CIDR for the vSwitch.
	vswitchCidr := "10.0.0.0/25"
	if !strings.HasPrefix(v.CidrBlock, "10.0.0.") {
		vswitchCidr = v.CidrBlock
	}

	result, err := vpcClient.CreateVSwitch(&vpc.CreateVSwitchRequest{
		RegionId:    tea.String(v.Region),
		ZoneId:      tea.String(zoneID),
		CidrBlock:   tea.String(vswitchCidr),
		VpcId:       tea.String(v.ID),
		VSwitchName: tea.String(v.BaseName + "-vswitch"),
		Tag:         vswitchNameTags(v.BaseName + "-vswitch"),
	})
	if err != nil {
		return err
	}
	v.VSwitchID = tea.StringValue(result.Body.VSwitchId)

	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		desc, err := vpcClient.DescribeVSwitchAttributes(&vpc.DescribeVSwitchAttributesRequest{
			VSwitchId: tea.String(v.VSwitchID),
			RegionId:  tea.String(v.Region),
		})
		if err != nil {
			return err
		}
		if tea.StringValue(desc.Body.Status) == "Available" {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for vSwitch %s to become Available", v.VSwitchID)
}

func (v *Vpc) setupSecurityGroup(ecsClient *ecs.Client) error {
	if len(v.IngressCIDRs) == 0 {
		return fmt.Errorf("security group ingress CIDRs not set")
	}

	sg, err := ecsClient.CreateSecurityGroup(&ecs.CreateSecurityGroupRequest{
		RegionId:          tea.String(v.Region),
		VpcId:             tea.String(v.ID),
		SecurityGroupName: tea.String(v.BaseName + "-sg"),
		Description:       tea.String("cloud-api-adaptor e2e tests"),
		Tag:               ecsNameTag(v.BaseName + "-sg"),
	})
	if err != nil {
		return err
	}
	v.SecurityGroupID = tea.StringValue(sg.Body.SecurityGroupId)

	rules := []struct {
		protocol string
		port     string
		desc     string
	}{
		{"icmp", "-1/-1", "ingress rule for icmp access"},
		{"tcp", "22/22", "ingress rule for ssh access"},
		{"tcp", "6443/6443", "ingress rule for https traffic"},
		{"tcp", "15150/15150", "ingress rule for CAA proxy traffic"},
	}

	for _, cidr := range v.IngressCIDRs {
		for _, rule := range rules {
			if _, err := ecsClient.AuthorizeSecurityGroup(&ecs.AuthorizeSecurityGroupRequest{
				RegionId:        tea.String(v.Region),
				SecurityGroupId: tea.String(v.SecurityGroupID),
				IpProtocol:      tea.String(rule.protocol),
				PortRange:       tea.String(rule.port),
				SourceCidrIp:    tea.String(cidr),
				Description:     tea.String(rule.desc),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *Vpc) deleteSecurityGroup(ecsClient *ecs.Client) error {
	if v.SecurityGroupID == "" {
		return nil
	}
	_, err := ecsClient.DeleteSecurityGroup(&ecs.DeleteSecurityGroupRequest{
		RegionId:        tea.String(v.Region),
		SecurityGroupId: tea.String(v.SecurityGroupID),
	})
	return err
}

func (v *Vpc) deleteVSwitch(ecsClient *ecs.Client, vpcClient *vpc.Client) error {
	if v.VSwitchID == "" {
		return nil
	}
	if err := v.drainVSwitch(ecsClient, vpcClient, v.VSwitchID); err != nil {
		return err
	}
	v.VSwitchID = ""
	return nil
}

func (v *Vpc) drainVSwitch(ecsClient *ecs.Client, vpcClient *vpc.Client, vswitchID string) error {
	desc, err := ecsClient.DescribeInstances(&ecs.DescribeInstancesRequest{
		RegionId:  tea.String(v.Region),
		VSwitchId: tea.String(vswitchID),
	})
	if err != nil {
		return err
	}

	if desc.Body != nil && desc.Body.Instances != nil {
		for _, inst := range desc.Body.Instances.Instance {
			id := tea.StringValue(inst.InstanceId)
			log.Infof("Delete instance %s (status %s)", id, tea.StringValue(inst.Status))
			_, _ = ecsClient.DeleteInstance(&ecs.DeleteInstanceRequest{
				InstanceId: tea.String(id),
				Force:      tea.Bool(true),
			})
		}

		deadline := time.Now().Add(10 * time.Minute)
		for time.Now().Before(deadline) {
			desc, err = ecsClient.DescribeInstances(&ecs.DescribeInstancesRequest{
				RegionId:  tea.String(v.Region),
				VSwitchId: tea.String(vswitchID),
			})
			if err != nil {
				return err
			}
			remaining := 0
			if desc.Body != nil && desc.Body.Instances != nil {
				remaining = len(desc.Body.Instances.Instance)
			}
			if remaining == 0 {
				break
			}
			time.Sleep(10 * time.Second)
		}
	}

	if err := v.deleteLeftoverENIs(ecsClient, vswitchID); err != nil {
		log.Warnf("Failed to delete leftover ENIs on vSwitch %s: %v", vswitchID, err)
	}

	_, err = vpcClient.DeleteVSwitch(&vpc.DeleteVSwitchRequest{
		RegionId:  tea.String(v.Region),
		VSwitchId: tea.String(vswitchID),
	})
	if err != nil && !isNotFound(err) {
		return err
	}

	// DeleteVSwitch is asynchronous; DeleteVpc fails while the switch still exists.
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		attrs, attrErr := vpcClient.DescribeVSwitchAttributes(&vpc.DescribeVSwitchAttributesRequest{
			RegionId:  tea.String(v.Region),
			VSwitchId: tea.String(vswitchID),
		})
		gone := attrErr != nil && isNotFound(attrErr)
		if !gone && attrs != nil && attrs.Body != nil && tea.StringValue(attrs.Body.Status) == "" {
			gone = tea.StringValue(attrs.Body.VSwitchId) == ""
		}
		if gone {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for vSwitch %s to be deleted", vswitchID)
}

func (v *Vpc) deleteLeftoverENIs(ecsClient *ecs.Client, vswitchID string) error {
	enis, err := ecsClient.DescribeNetworkInterfaces(&ecs.DescribeNetworkInterfacesRequest{
		RegionId:  tea.String(v.Region),
		VSwitchId: tea.String(vswitchID),
	})
	if err != nil {
		return err
	}
	if enis.Body == nil || enis.Body.NetworkInterfaceSets == nil {
		return nil
	}
	for _, eni := range enis.Body.NetworkInterfaceSets.NetworkInterfaceSet {
		id := tea.StringValue(eni.NetworkInterfaceId)
		typ := tea.StringValue(eni.Type)
		if typ == "Primary" {
			continue
		}
		log.Infof("Delete leftover ENI %s on vSwitch %s", id, vswitchID)
		_, _ = ecsClient.DeleteNetworkInterface(&ecs.DeleteNetworkInterfaceRequest{
			RegionId:           tea.String(v.Region),
			NetworkInterfaceId: tea.String(id),
		})
	}
	return nil
}

func (v *Vpc) deleteVpc(vpcClient *vpc.Client, ecsClient *ecs.Client) error {
	if v.ID == "" {
		return nil
	}

	// Delete any remaining vSwitches before removing the VPC.
	vsw, err := vpcClient.DescribeVSwitches(&vpc.DescribeVSwitchesRequest{
		RegionId: tea.String(v.Region),
		VpcId:    tea.String(v.ID),
	})
	if err != nil {
		return err
	}
	if vsw.Body != nil && vsw.Body.VSwitches != nil {
		for _, vs := range vsw.Body.VSwitches.VSwitch {
			id := tea.StringValue(vs.VSwitchId)
			if id == "" {
				continue
			}
			log.Infof("Delete remaining vSwitch: %s", id)
			if err := v.drainVSwitch(ecsClient, vpcClient, id); err != nil {
				return err
			}
		}
	}

	_, err = vpcClient.DeleteVpc(&vpc.DeleteVpcRequest{
		RegionId: tea.String(v.Region),
		VpcId:    tea.String(v.ID),
	})
	if err != nil && !isNotFound(err) {
		deadline := time.Now().Add(2 * time.Minute)
		var lastErr error
		for time.Now().Before(deadline) {
			_, lastErr = vpcClient.DeleteVpc(&vpc.DeleteVpcRequest{
				RegionId: tea.String(v.Region),
				VpcId:    tea.String(v.ID),
			})
			if lastErr == nil || isNotFound(lastErr) {
				return nil
			}
			if !strings.Contains(lastErr.Error(), "dependent") && !strings.Contains(lastErr.Error(), "DependencyViolation") {
				return lastErr
			}
			log.Warnf("VPC %s still has dependents, retrying: %v", v.ID, lastErr)
			time.Sleep(10 * time.Second)
		}
		return lastErr
	}
	return nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "NotFound") || strings.Contains(msg, "InvalidVSwitchId") || strings.Contains(msg, "InvalidVpcId")
}

func NewAlibabaCloudInstallChart(installDir, provider string) (pv.InstallChart, error) {
	chartPath := filepath.Join(installDir, "charts", "peerpods")
	namespace := pv.GetCAANamespace()
	releaseName := "peerpods"
	debug := false

	helm, err := pv.NewHelm(chartPath, namespace, releaseName, provider, debug)
	if err != nil {
		return nil, err
	}

	return &AlibabaCloudInstallChart{
		Helm: helm,
	}, nil
}

func (a *AlibabaCloudInstallChart) GetHelm() *pv.Helm { return a.Helm }

func (a *AlibabaCloudInstallChart) Install(ctx context.Context, cfg *envconf.Config) error {
	return a.Helm.Install(ctx, cfg)
}

func (a *AlibabaCloudInstallChart) Uninstall(ctx context.Context, cfg *envconf.Config) error {
	return a.Helm.Uninstall(ctx, cfg)
}

func (a *AlibabaCloudInstallChart) Configure(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	if properties["CAA_IMAGE"] != "" {
		img := strings.Split(properties["CAA_IMAGE"], ":")
		a.Helm.OverrideValues["image.name"] = img[0]
		if len(img) == 2 {
			a.Helm.OverrideValues["image.tag"] = img[1]
		}
	}

	if properties["CONTAINER_RUNTIME"] == "crio" {
		a.Helm.OverrideValues["kata-deploy.snapshotter.setup"] = ""
	}

	mapProps := map[string]string{
		"disablecvm":          "DISABLECVM",
		"pause_image":         "PAUSE_IMAGE",
		"podvm_image_id":      "IMAGEID",
		"podvm_instance_type": "PODVM_INSTANCE_TYPE",
		"security_group_ids":  "SECURITY_GROUP_IDS",
		"vswitch_id":          "VSWITCH_ID",
		"ssh_kp_name":         "KEYNAME",
		"region":              "REGION",
		"tunnel_type":         "TUNNEL_TYPE",
		"vxlan_port":          "VXLAN_PORT",
		"use_public_ip":       "USE_PUBLIC_IP",
	}

	for k, v := range mapProps {
		if properties[k] != "" {
			a.Helm.OverrideProviderValues[v] = properties[k]
		}
	}

	if properties["peerpods_secret_name"] == "" {
		if properties["access_key_id"] != "" {
			a.Helm.OverrideProviderSecrets["ALIBABACLOUD_ACCESS_KEY_ID"] = properties["access_key_id"]
		}
		if properties["secret_access_key"] != "" {
			a.Helm.OverrideProviderSecrets["ALIBABACLOUD_ACCESS_KEY_SECRET"] = properties["secret_access_key"]
		}
		if properties["security_token"] != "" {
			a.Helm.OverrideProviderSecrets["ALIBABACLOUD_SECURITY_TOKEN"] = properties["security_token"]
		}
	} else {
		a.Helm.OverrideValues["secrets.mode"] = "reference"
		a.Helm.OverrideValues["secrets.existingSecretName"] = properties["peerpods_secret_name"]
	}

	return nil
}

func NewOnPremCluster() *OnPremCluster {
	return &OnPremCluster{}
}

func (o *OnPremCluster) CreateCluster() error {
	log.Info("On-prem cluster type selected. Nothing to do.")
	return nil
}

func (o *OnPremCluster) DeleteCluster() error {
	log.Info("On-prem cluster type selected. Nothing to do.")
	return nil
}

func (o *OnPremCluster) GetKubeconfigFile() (string, error) {
	kubeconfigPath := kconf.ResolveKubeConfigFile()
	if kubeconfigPath == "" {
		return "", fmt.Errorf("unable to find a kubeconfig file")
	}
	return kubeconfigPath, nil
}
