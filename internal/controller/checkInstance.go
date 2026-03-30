package controller

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	computev1 "github.com/vareja0/operator-repo/api/v1"
)

// describeLiveInstance returns the current state and relevant spec fields of a running EC2 instance.
func describeLiveInstance(ctx context.Context, region, instanceID string) (*computev1.LiveInstanceSpec, error) {
	client, err := newEC2Client(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("failed to create EC2 client: %w", err)
	}

	out, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance %s: %w", instanceID, err)
	}

	if len(out.Reservations) == 0 || len(out.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}

	inst := out.Reservations[0].Instances[0]

	subnetID := ""
	if len(inst.NetworkInterfaces) > 0 {
		subnetID = aws.ToString(inst.NetworkInterfaces[0].SubnetId)
	}

	return &computev1.LiveInstanceSpec{
		State:         string(inst.State.Name),
		InstanceType:  string(inst.InstanceType),
		AmiID:         aws.ToString(inst.ImageId),
		SshKey:        aws.ToString(inst.KeyName),
		AvailableZone: aws.ToString(inst.Placement.AvailabilityZone),
		SubnetID:      subnetID,
	}, nil
}

// specChanged returns true if the desired spec differs from the live instance.
func specChanged(live *computev1.LiveInstanceSpec, desired *computev1.EC2instanceSpec) bool {
	return live.InstanceType != desired.Type ||
		live.AmiID != desired.AmiID ||
		live.SshKey != desired.SshKey ||
		live.AvailableZone != desired.AvaibilityZone ||
		live.SubnetID != desired.Subnet
}
