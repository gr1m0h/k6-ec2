package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/k6-distributed/k6-aws-common/pkg/types"
	"github.com/k6-distributed/k6-ec2/internal/config"
)

const DefaultStateFile = ".k6-ec2-state.json"

// PipelineState holds the inter-subcommand state persisted as JSON.
type PipelineState struct {
	TestName      string                  `json:"testName"`
	Region        string                  `json:"region"`
	ScriptS3      *types.S3Location       `json:"scriptS3,omitempty"`
	AMI           string                  `json:"ami,omitempty"`
	Instances     []config.InstanceStatus `json:"instances,omitempty"`
	SpotCount     int                     `json:"spotCount,omitempty"`
	FallbackCount int                     `json:"fallbackCount,omitempty"`
	LogGroup      string                  `json:"logGroup"`
	Phase         string                  `json:"phase"`
	CreatedAt     time.Time               `json:"createdAt"`
	UpdatedAt     time.Time               `json:"updatedAt"`
}

// InstanceIDs extracts instance IDs from the state.
func (s *PipelineState) InstanceIDs() []string {
	ids := make([]string, 0, len(s.Instances))
	for _, inst := range s.Instances {
		ids = append(ids, inst.InstanceID)
	}
	return ids
}

// LoadState reads a PipelineState from a JSON file.
func LoadState(path string) (*PipelineState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file %s: %w", path, err)
	}
	var state PipelineState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}
	return &state, nil
}

// SaveState writes the PipelineState to a JSON file.
func SaveState(path string, state *PipelineState) error {
	state.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file %s: %w", path, err)
	}
	return nil
}
