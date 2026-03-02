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

// Prepare resolves the test script (uploading to S3 if needed) and the AMI.
// It returns a PrepareResult that can be passed to Launch.
func (r *Runner) Prepare(ctx context.Context) (*PrepareResult, error) {
	r.setPhase(types.PhaseCreating)

	loc, err := r.scripts.Resolve(ctx, &r.spec.Spec.Script, r.spec.Metadata.Name)
	if err != nil {
		return nil, fmt.Errorf("script preparation failed: %w", err)
	}
	r.scriptS3 = loc
	r.logger.Info("script ready", "bucket", loc.Bucket, "key", loc.Key)

	ami := r.spec.Spec.Runner.AMI
	if ami == "" {
		ami, err = r.resolveLatestAMI(ctx)
		if err != nil {
			return nil, fmt.Errorf("AMI resolution failed: %w", err)
		}
		r.logger.Info("resolved AMI", "ami", ami)
	}

	return &PrepareResult{
		ScriptS3: loc,
		AMI:      ami,
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
