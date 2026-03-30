package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// deleteEc2Instance terminates the EC2 instance with the given ID.
func deleteEc2Instance(ctx context.Context, region, instanceID, endpoint string) error {
	client, err := newEC2Client(ctx, region, endpoint)
	if err != nil {
		return fmt.Errorf("failed to create EC2 client: %w", err)
	}

	_, err = client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("failed to terminate instance %s: %w", instanceID, err)
	}

	waiter := ec2.NewInstanceTerminatedWaiter(client)
	if err := waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}, 3*time.Minute); err != nil {
		return fmt.Errorf("instance %s did not reach terminated state: %w", instanceID, err)
	}

	return nil
}
