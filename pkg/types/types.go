package types

import (
	"fmt"
	"time"
)

// ScriptSpec defines the k6 test script source.
type ScriptSpec struct {
	LocalFile string `yaml:"localFile,omitempty" json:"localFile,omitempty"`
	S3        string `yaml:"s3,omitempty" json:"s3,omitempty"`
	Inline    string `yaml:"inline,omitempty" json:"inline,omitempty"`
}

// OutputSpec defines output configuration for k6 metrics.
type OutputSpec struct {
	StatSD *StatsDSpec `yaml:"statsd,omitempty" json:"statsd,omitempty"`
}

// StatsDSpec defines StatsD output configuration.
type StatsDSpec struct {
	Address     string `yaml:"address,omitempty" json:"address,omitempty"`
	EnabledTags bool   `yaml:"enabledTags,omitempty" json:"enabledTags,omitempty"`
	Namespace   string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
}

// S3Location represents an object in S3.
type S3Location struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

// TestRunPhase represents the current phase of a test run.
type TestRunPhase string

const (
	PhaseInitializing TestRunPhase = "initializing"
	PhaseCreating     TestRunPhase = "creating"
	PhaseStarted      TestRunPhase = "started"
	PhaseRunning      TestRunPhase = "running"
	PhaseFinishing    TestRunPhase = "finishing"
	PhaseCompleted    TestRunPhase = "completed"
	PhaseFailed       TestRunPhase = "failed"
	PhaseCancelled    TestRunPhase = "cancelled"
)

// RunnerResult holds the result of a single runner instance.
type RunnerResult struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Status   string `json:"status"`
	ExitCode *int   `json:"exitCode,omitempty"`
}

// SpotInfo holds Spot Instance usage information.
type SpotInfo struct {
	Used     bool `json:"used"`
	Count    int  `json:"count"`
	Fallback int  `json:"fallback"`
}

// ValidateName validates the test run name.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// ValidateScript validates the script specification.
func ValidateScript(s *ScriptSpec) error {
	count := 0
	if s.LocalFile != "" {
		count++
	}
	if s.S3 != "" {
		count++
	}
	if s.Inline != "" {
		count++
	}
	if count == 0 {
		return fmt.Errorf("one of localFile, s3, or inline is required")
	}
	if count > 1 {
		return fmt.Errorf("only one of localFile, s3, or inline can be specified")
	}
	return nil
}

// ValidateParallelism validates the parallelism value.
func ValidateParallelism(p int32) error {
	if p < 1 {
		return fmt.Errorf("parallelism must be >= 1, got %d", p)
	}
	if p > 100 {
		return fmt.Errorf("parallelism must be <= 100, got %d", p)
	}
	return nil
}

// ValidateTimeout validates a timeout duration string.
func ValidateTimeout(t string) error {
	if t == "" {
		return nil
	}
	_, err := time.ParseDuration(t)
	if err != nil {
		return fmt.Errorf("invalid timeout %q: %w", t, err)
	}
	return nil
}

// ValidateCleanup validates the cleanup policy string.
func ValidateCleanup(policy string) error {
	switch policy {
	case "", "always", "on-success", "never":
		return nil
	default:
		return fmt.Errorf("invalid cleanup policy %q: must be always, on-success, or never", policy)
	}
}
