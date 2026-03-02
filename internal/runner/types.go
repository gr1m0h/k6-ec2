package runner

import (
	"github.com/gr1m0h/k6-ec2/internal/config"
	"github.com/gr1m0h/k6-ec2/pkg/types"
)

// PrepareResult contains the outputs of the prepare phase.
type PrepareResult struct {
	ScriptS3 *types.S3Location `json:"scriptS3"`
	AMI      string            `json:"ami"`
}

// LaunchParams contains the inputs for the launch phase.
type LaunchParams struct {
	AMI      string            `json:"ami"`
	ScriptS3 *types.S3Location `json:"scriptS3"`
}

// LaunchResult contains the outputs of the launch phase.
type LaunchResult struct {
	Instances     []config.InstanceStatus `json:"instances"`
	SpotCount     int                     `json:"spotCount"`
	FallbackCount int                     `json:"fallbackCount"`
}

// ExecuteParams contains the inputs for the execute phase.
type ExecuteParams struct {
	InstanceIDs       []string          `json:"instanceIds"`
	ScriptS3          *types.S3Location `json:"scriptS3,omitempty"`
	ExternalInstances bool              `json:"externalInstances,omitempty"`
}

// ExecuteResult contains the outputs of the execute phase.
type ExecuteResult struct {
	Instances []config.InstanceStatus `json:"instances"`
	CommandID string                  `json:"commandId,omitempty"`
}

// CleanupParams contains the inputs for the cleanup phase.
type CleanupParams struct {
	InstanceIDs []string `json:"instanceIds"`
	Force       bool     `json:"force,omitempty"`
}
