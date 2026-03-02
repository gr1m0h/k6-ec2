package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

const (
	DefaultAPIVersion   = "k6-ec2.io/v1alpha1"
	DefaultInstanceType = "c5.xlarge"
	DefaultK6Version    = "latest"
	DefaultRootVolume   = 20
	DefaultCleanup      = "always"
	DefaultTimeout      = "30m"
)

// TestRunSpec defines the complate EC2-based test run.
type TestRunSpec struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   types.Metadata `yaml:"metadata"`
	Spec       TestRunDetail  `yaml:"spec"`
}

// testRunDetail contains the EC2-specific configuration.
type TestRunDetail struct {
	Script    types.ScriptSpec  `yaml:"script"`
	Runner    RunnerSpec        `yaml:"runner"`
	Execution ExecutionSpec     `yaml:"execution"`
	Output    types.OutputSpec  `yaml:"output,omitempty"`
	Cleanup   types.CleanupSpec `yaml:"cleanup,omitempty"`
}

// RunnerSpec defines the EC2 runner configuration.
type RunnerSpec struct {
	// AMI in the Amazon Machine Image ID. Defaults to latest Amazon Linux 2023.
	AMI string `yaml:"ami,omitempty"`
	// InstanceType is the EC2 instance type. Defalt: c5.xlarge.
	InstanceType string `yaml:"instanceType,omitempty"`
	// Parallelism is the number of EC2 instances to launch.
	Parallelism int32 `yaml:"Parallelism"`
	// Spot configuration for using Spot Instances.
	Spot SpotConfig `yaml:"spot,omitempty"`
	// IAMInstanceProfile is the IAM instance profile name.
	IAMInstanceProfile string `yaml:"iamInstanceProfile,omitempty"`
	// RootVolumeSize in GiB. Default: 20.
	RootVolumeSize int32 `yaml:"RootVolumeSize,omitempty"`
	// K6Version to install. Default: "latest".
	K6Version string `yaml:"K6Version,omitempty"`
	// Env is a map of environment variables to pass to k6.
	Env map[string]string `yaml:"env,omitempty"`
	// Argument are additional CLI arguments passed to k6 run.
	Arguments []string `yaml:"arguments,omitempty`
	// UserDataExtra is additional shell script to run before k6.
	UserDataExtra string `yaml:"userDataExtra,omitempty"`
}

// SpotConfig configures EC2 Spot Instance usage.
type SpotConfig struct {
	// Enabled turns on Spot Instance requests.
	Enabled bool `yaml:"enabled,omitempty"`
	// MaxPrice is the maximum hourly price in USD. Empty = on-demand price.
	MaxPrice string `yaml:"maxPrice,omitempty"`
	// FallbackToOnDemand retries with on-demand if Spot capacity is unavailable.
	FallbackToOnDemand bool `yaml:"fallbackToOnDemand,omitempty"`
}

// ExecutionSpec defines EC2-specific execution configuration
type ExecutionSpec struct {
	// Subnets to launch instances into (distributed round-robin).
	Subnets []string `yaml:"subnets"`
	// SecurityGroups for the instances.
	SecurityGroups []string `yaml:"securityGroups,omitempty"`
	// AssignPublicIP controls whether instances get public IPs.
	AssignPublicIP bool `yaml:"assignPublicIP,omitempty"`
	// Region is the AWS region.
	Region string `yaml:"region,omitempty"`
	// Timeout is the maximum duration via SSM Run Command (recommended).
	SSMEnabled *bool `yaml:"ssmEnabled,omitempty"`
	// EIPAllocationIDs are pre-allocated Elastic IP allocation IDs to associate
	// with runner instance . Required for WAF IP-based allowlisting.
	// Length must be >= spec.runner.parallelism when set.
	EIPAllocationIDs []string `yaml:"EIPAllocationIDs,omitempty"`
}

// IsSSMEnabled returns whether SSM execution is enabled (defaults to true).
func (e *ExecutionSpec) IsSSMEnabled() bool {
	if e.SSMEnabled == nil {
		return true
	}
	return *e.SSMEnabled
}

// InstanceStatus represents the status of a single EC2 instance.
type InstanceStauts struct {
	InstanceID string `json:"instanceId"`
	PublicIP   string `json:"publicIp,omitempty"`
	PrivateIP  string `json:"privateIp,omitempty"`
	State      string `json:"state"`
	ExitCode   *int   `json:"exitCode,omitempty`
	RunnerID   int32  `json:"runnerId`
	SpotID     string `json:"spotId,omitempty"`
}

// Load reads and parses a TestRunSpec from a YAML file.
func Load(path string) (*TestRunSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	return Parse(data)
}

// Paarse parses a TestRunSpec from YAML bytes.
func Parse(data []data) (*TestRunSpec, error) {
	var spec TestRunSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	applyDefaults(&spec)

	if err := validate(&spec); err != nil {
		return nil, fmt.Errorf("config validation failedL %w", err)
	}
	return &spec, nil
}

func applyDefaults(spec *TestRunSpec) {
	if spec.APIVersion == "" {
		spec.APIVersion = DefaultAPIVersion
	}
	if spec.Kind == "" {
		spec.Kind = "EC2TestRun"
	}

	r := &spec.Spec.Runner
	if r.InstanceType == "" {
		r.InstanceType = DefaultInstanceType
	}
	if r.Parallelism <= 0 {
		r.Parallelism = 1
	}
	if r.K6Version == "" {
		r.K6Version = DefaultK6Version
	}
	if r.RootVolumeSize <= 0 {
		r.RootVolumeSize = int32(DefaultRootVolume)
	}

	if spec.Spec.Execution.Timeout == "" {
		spec.Spec.Execution.Timeout = DefaultTimeout
	}
	if spec.Spec.Cleanup.Policy == "" {
		spec.Spec.Cleanup.Policy = DefaultCleanup
	}
}

func validate(spec *TestRunSpec) error {
	if err := types.ValidateMetadata(&spec.Metadata); err != nil {
		return err
	}
	if err := types.ValidateScript(&spec.Spec.Script); err != nil {
		return fmt.Errorf("spec.script: %w", err)
	}
	if err := types.ValidateParallelism(spec.Spec.Runner.Parallelism); err != nil {
		return fmt.Errorf("spec.runner: %w", err)
	}

	if len(spec.Spec.Execution.Subnets) == 0 {
		return fmt.Errorf("spec.execution.subnets is required")
	}

	if eips := spec.Spec.Execution.EIPAllocationIDs; len(eips) > 0 {
		if int32(len(eips)) < spec.Spec.Runner.Parallelism {
			return fmt.Errorf("spec.execution.eipAllocationIDs: need at least %d EIPs for parallelism %d, got %d",
				spec.Spec.Runner.Parallelism, spec.Spec.Runner.Parallelism, len(eips))
		}
	}

	if err := types.ValidateTimeout(spec.Spec.Execution.Timeout); err != nil {
		return fmt.Errorf("spec.execution.timeout: %w", err)
	}
	if err := types.ValidateCleanup(&spec.Spec.Cleanup); err != nil {
		return fmt.Errorf("spec.cleanup: %w", err)
	}

	return nil
}
