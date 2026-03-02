package runner

import (
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gr1m0h/k6-ec2/pkg/types"
)

// Prepare resolves the AMI for EC2 instances.
// It returns a PrepareResult that can be passed to Launch.
func (r *Runner) Prepare(ctx context.Context) (*PrepareResult, error) {
	r.setPhase(types.PhaseCreating)

	ami := r.spec.Runner.AMI
	if ami == "" {
		var err error
		ami, err = r.resolveLatestAMI(ctx)
		if err != nil {
			return nil, fmt.Errorf("AMI resolution failed: %w", err)
		}
		r.logger.Info("resolved AMI", "ami", ami)
	}

	return &PrepareResult{
		AMI: ami,
	}, nil
}

func (r *Runner) resolveLatestAMI(ctx context.Context) (string, error) {
	res, err := r.ec2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{"amazon"},
		Filters: []ec2types.Filter{
			{Name: aws.String("name"), Values: []string{"al2023-ami-2023.*-x86_64"}},
			{Name: aws.String("state"), Values: []string{"available"}},
			{Name: aws.String("architecture"), Values: []string{"x86_64"}},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe images: %w", err)
	}
	if len(res.Images) == 0 {
		return "", fmt.Errorf("no Amazon Linux 2023 AMI found")
	}

	sort.Slice(res.Images, func(i, j int) bool {
		return aws.ToString(res.Images[i].CreationDate) > aws.ToString(res.Images[j].CreationDate)
	})

	return aws.ToString(res.Images[0].ImageId), nil
}
