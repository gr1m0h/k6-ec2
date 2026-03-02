package script

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gr1m0h/k6-ec2/pkg/types"
)

// Manager handles k6 test script resolution and upload to S3.
type Manager struct {
	s3Client *s3.Client
	bucket   string
	prefix   string
}

// NewManager creates a new script Manager.
func NewManager(s3Client *s3.Client, bucket, prefix string) *Manager {
	return &Manager{
		s3Client: s3Client,
		bucket:   bucket,
		prefix:   prefix,
	}
}

// Resolve resolves the test script and uploads it to S3 if needed.
// Returns the S3 location of the script.
func (m *Manager) Resolve(ctx context.Context, spec *types.ScriptSpec, testName string) (*types.S3Location, error) {
	key := fmt.Sprintf("%s/%s/test.js", m.prefix, testName)
	loc := &types.S3Location{Bucket: m.bucket, Key: key}

	if spec.S3 != "" {
		parsed, err := parseS3URI(spec.S3)
		if err != nil {
			return nil, fmt.Errorf("invalid S3 script URI: %w", err)
		}
		return parsed, nil
	}

	var content []byte
	if spec.LocalFile != "" {
		var err error
		content, err = os.ReadFile(spec.LocalFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read script file %s: %w", spec.LocalFile, err)
		}
	} else if spec.Inline != "" {
		content = []byte(spec.Inline)
	} else {
		return nil, fmt.Errorf("no script source specified")
	}

	_, err := m.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(content),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload script to S3: %w", err)
	}

	return loc, nil
}

func parseS3URI(uri string) (*types.S3Location, error) {
	if !strings.HasPrefix(uri, "s3://") {
		return nil, fmt.Errorf("invalid S3 URI %q: must start with s3://", uri)
	}
	rest := strings.TrimPrefix(uri, "s3://")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid S3 URI %q: expected s3://bucket/key", uri)
	}
	return &types.S3Location{
		Bucket: parts[0],
		Key:    parts[1],
	}, nil
}
