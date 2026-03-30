package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	computev1 "github.com/vareja0/operator-repo/api/v1"
)

func createEc2Instance(ctx context.Context, ec2Instance *computev1.EC2instance, endpoint string) (*computev1.CreatedInstanceInfo, error) {
	l := logf.FromContext(ctx)
	l.Info("Starting EC2 instance creation", "instanceName", ec2Instance.Spec.InstanceName, "region", ec2Instance.Spec.Region)

	// initialize the EC2 client using credentials from env
	client, err := newEC2Client(ctx, ec2Instance.Spec.Region, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create EC2 client: %w", err)
	}

	// build block device mappings from the root volume and any additional volumes
	var blockDeviceMappings []types.BlockDeviceMapping

	if ec2Instance.Spec.Storage.RootVoume.Size > 0 {
		blockDeviceMappings = append(blockDeviceMappings, types.BlockDeviceMapping{
			DeviceName: aws.String(ec2Instance.Spec.Storage.RootVoume.DeviceName),
			Ebs: &types.EbsBlockDevice{
				VolumeSize: aws.Int32(ec2Instance.Spec.Storage.RootVoume.Size),
				VolumeType: types.VolumeType(ec2Instance.Spec.Storage.RootVoume.Type),
				Encrypted:  aws.Bool(ec2Instance.Spec.Storage.RootVoume.Encrypted),
			},
		})
	}

	for _, vol := range ec2Instance.Spec.Storage.AdditionalVolumes {
		blockDeviceMappings = append(blockDeviceMappings, types.BlockDeviceMapping{
			DeviceName: aws.String(vol.DeviceName),
			Ebs: &types.EbsBlockDevice{
				VolumeSize: aws.Int32(vol.Size),
				VolumeType: types.VolumeType(vol.Type),
				Encrypted:  aws.Bool(vol.Encrypted),
			},
		})
	}

	// always include the Name tag, then append any extra tags from the spec
	awsTags := []types.Tag{
		{Key: aws.String("Name"), Value: aws.String(ec2Instance.Spec.InstanceName)},
	}
	for k, v := range ec2Instance.Spec.Tags {
		awsTags = append(awsTags, types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}

	// AWS requires user data to be base64-encoded
	var userDataEncoded *string
	if ec2Instance.Spec.UserData != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(ec2Instance.Spec.UserData))
		userDataEncoded = aws.String(encoded)
	}

	// NetworkInterfaces is used instead of top-level SubnetId/SecurityGroupIds
	// because AssociatePublicIpAddress can only be set at the interface level
	input := &ec2.RunInstancesInput{
		ImageId:             aws.String(ec2Instance.Spec.AmiID),
		InstanceType:        types.InstanceType(ec2Instance.Spec.Type),
		KeyName:             aws.String(ec2Instance.Spec.SshKey),
		MinCount:            aws.Int32(1),
		MaxCount:            aws.Int32(1),
		BlockDeviceMappings: blockDeviceMappings,
		UserData:            userDataEncoded,
		Placement: &types.Placement{
			AvailabilityZone: aws.String(ec2Instance.Spec.AvaibilityZone),
		},
		NetworkInterfaces: []types.InstanceNetworkInterfaceSpecification{
			{
				AssociatePublicIpAddress: aws.Bool(ec2Instance.Spec.AssociatePublicIP),
				DeviceIndex:              aws.Int32(0),
				SubnetId:                 aws.String(ec2Instance.Spec.Subnet),
				Groups:                   ec2Instance.Spec.SecurityGroups,
			},
		},
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags:         awsTags,
			},
		},
	}

	l.Info("=== CALLING AWS RunInstances API ===")

	result, err := client.RunInstances(ctx, input)
	if err != nil {
		l.Error(err, "Failed to create EC2 instance")
		return nil, fmt.Errorf("Failed to create EC2 instances: %w", err)
	}

	if len(result.Instances) == 0 {
		l.Error(nil, "No instances returned in RunInstancesOutput")
		fmt.Println("No instances returned in RunInstancesOutput")
		return nil, nil
	}

	instanceID := aws.ToString(result.Instances[0].InstanceId)
	l.Info("Instance launched, waiting for running state", "instanceID", instanceID)

	// poll until the instance reaches the running state (up to 3 minutes)
	waiter := ec2.NewInstanceRunningWaiter(client)
	if err := waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}, 3*time.Minute); err != nil {
		return nil, fmt.Errorf("instance %s did not reach running state: %w", instanceID, err)
	}

	l.Info("Instance is running", "instanceID", instanceID)

	// fetch the full instance details now that it is running
	describeInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	described, err := client.DescribeInstances(ctx, describeInput)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance %s: %w", instanceID, err)
	}

	instance := described.Reservations[0].Instances[0]

	return &computev1.CreatedInstanceInfo{
		InstanceID: aws.ToString(instance.InstanceId),
		PublicIP:   aws.ToString(instance.PublicIpAddress),
		PrivateIP:  aws.ToString(instance.PrivateIpAddress),
		PublicDNS:  aws.ToString(instance.PublicDnsName),
		PrivateDNS: aws.ToString(instance.PrivateDnsName),
		State:      string(instance.State.Name),
	}, nil
}
