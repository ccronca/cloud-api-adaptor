// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
	"golang.org/x/oauth2/google"
	option "google.golang.org/api/option"
	proto "google.golang.org/protobuf/proto"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/gcp] ", log.LstdFlags|log.Lmsgprefix)
var computeScope = "https://www.googleapis.com/auth/compute"

const maxInstanceNameLen = 63

type gcpProvider struct {
	serviceConfig   *Config
	instancesClient *compute.InstancesClient
}

func NewProvider(config *Config) (cloud.Provider, error) {
	logger.Printf("gcp config: %#v", config.Redact())
	provider := &gcpProvider{
		serviceConfig:   config,
		instancesClient: nil,
	}
	if config.GcpCredentials != "" {
		creds, err := google.CredentialsFromJSON(context.TODO(), []byte(config.GcpCredentials), computeScope)
		if err != nil {
			return nil, fmt.Errorf("configuration error when using creds: %s", err)
		}
		provider.instancesClient, err = compute.NewInstancesRESTClient(context.TODO(), option.WithCredentials(creds))
		if err != nil {
			return nil, fmt.Errorf("NewInstancesRESTClient with credentials error: %s", err)
		}
	} else {
		var err error
		provider.instancesClient, err = compute.NewInstancesRESTClient(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("NewInstancesRESTClient error: %s", err)
		}
	}
	return provider, nil
}

func (p *gcpProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec cloud.InstanceTypeSpec) (*cloud.Instance, error) {

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)
	logger.Printf("CreateInstance: name: %q", instanceName)

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	//Convert userData to base64
	userDataEnc := base64.StdEncoding.EncodeToString([]byte(userData))
	logger.Printf("userDataEnc:  %s", userDataEnc)

	insertReq := &computepb.InsertInstanceRequest{
		Project: p.serviceConfig.ProjectId,
		Zone:    p.serviceConfig.Zone,
		InstanceResource: &computepb.Instance{
			Name: proto.String(instanceName),
			Disks: []*computepb.AttachedDisk{
				{
					InitializeParams: &computepb.AttachedDiskInitializeParams{
						DiskSizeGb:  proto.Int64(20),
						SourceImage: proto.String(fmt.Sprintf("projects/%s/global/images/%s", p.serviceConfig.ProjectId, p.serviceConfig.ImageName)),
						DiskType:    proto.String(fmt.Sprintf("zones/%s/diskTypes/pd-standard", p.serviceConfig.Zone)),
					},
					AutoDelete: proto.Bool(true),
					Boot:       proto.Bool(true),
					Type:       proto.String(computepb.AttachedDisk_PERSISTENT.String()),
				},
			},
			Metadata: &computepb.Metadata{
				Items: []*computepb.Items{
					{
						Key:   proto.String("user-data"),
						Value: proto.String(userDataEnc),
					},
					{
						Key:   proto.String("user-data-encoding"),
						Value: proto.String("base64"),
					},
				},
			},
			MachineType: proto.String(fmt.Sprintf("zones/%s/machineTypes/%s", p.serviceConfig.Zone, p.serviceConfig.MachineType)),
			NetworkInterfaces: []*computepb.NetworkInterface{
				{
					AccessConfigs: []*computepb.AccessConfig{
						{
							Name:        proto.String("External NAT"),
							NetworkTier: proto.String("PREMIUM"),
						},
					},
					StackType: proto.String("IPV4_Only"),
					Name:      proto.String(p.serviceConfig.Network),
				},
			},
		},
	}
	op, err := p.instancesClient.Insert(ctx, insertReq)
	if err != nil {
		return nil, fmt.Errorf("Instances.Insert error: %s. req: %v", err, insertReq)
	}
	err = op.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("waiting for Instances.Insert error: %s. req: %v", err, insertReq)
	}
	logger.Printf("created an instance %s for sandbox %s", instanceName, sandboxID)

	getReq := &computepb.GetInstanceRequest{
		Project:  p.serviceConfig.ProjectId,
		Zone:     p.serviceConfig.Zone,
		Instance: instanceName,
	}

	instance, err := p.instancesClient.Get(ctx, getReq)
	if err != nil {
		return nil, fmt.Errorf("unable to get instance: %w, req: %v", err, getReq)
	}
	logger.Printf("instance name %s, id %d", instance.GetName(), instance.GetId())

	var ips []net.IP
	for _, nic := range instance.GetNetworkInterfaces() {
		for _, access := range nic.GetAccessConfigs() {
			ipStr := access.GetNatIP()
			logger.Printf("ip %s", ipStr)
			ip := net.ParseIP(ipStr)
			if ip == nil {
				return nil, fmt.Errorf("failed to parse pod node IP %q", ipStr)
			}
			ips = append(ips, ip)
		}
	}

	return &cloud.Instance{
		ID:   instance.GetName(),
		Name: instance.GetName(),
		IPs:  nil, // TODO: ips,
	}, nil
}

func (p *gcpProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	req := &computepb.DeleteInstanceRequest{
		Project:  p.serviceConfig.ProjectId,
		Zone:     p.serviceConfig.Zone,
		Instance: instanceID,
	}
	op, err := p.instancesClient.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("Instances.Delete error: %w, req: %v", err, req)
	}
	err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("waiting for Instances.Delete error: %s. req: %v", err, req)
	}
	logger.Printf("deleted an instance %s", instanceID)
	return nil
}

func (p *gcpProvider) ConfigVerifier() error {
	ImageName := p.serviceConfig.ImageName
	if len(ImageName) == 0 {
		return fmt.Errorf("ImageName is empty")
	}
	return nil
}

func (p *gcpProvider) Teardown() error {
	return nil
}
