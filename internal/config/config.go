package config

import (
	"fmt"
	"os"

	"github.com/gr1m0h/k6-ec2/pkg/types"
	"gopkg.in/yaml.v2"
)

const (
	DefaultInstanceType = "c5.xlarge"
	DefaultK6Version    = "latest"
	DefaultRootVolume   = 20
	DefaultCleanup      = "always"
	DefaultTimeout      = "30m"
)

// Config defines the complete EC2-based test run configuration.
type Config struct {
	Name      string            `yaml:"name"`
	Labels    map[string]string `yaml:"labels,omitempty"`
	Script    types.ScriptSpec  `yaml:"script"`
	Runner    RunnerSpec        `yaml:"runner"`
	Execution ExecutionSpec     `yaml:"execution"`
	Output    types.OutputSpec  `yaml:"output,omitempty"`
	Cleanup   string            `yaml:"cleanup,omitempty"`
}

// RunnerSpec defines the EC2 runner configuration.
type RunnerSpec struct {
	// AMI is the Amazon Machine Image ID. Defaults to latest Amazon Linux 2023.
	AMI string `yaml:"ami,omitempty"`
	// InstanceType is the EC2 instance type. Default: c5.xlarge.
	InstanceType string `yaml:"instanceType,omitempty"`
	// Parallelism is the number of EC2 instances to launch.
	Parallelism int32 `yaml:"parallelism"`
	// Spot configuration for using Spot Instances.
	Spot SpotConfig `yaml:"spot,omitempty"`
	// IAMInstanceProfile is the IAM instance profile name.
	IAMInstanceProfile string `yaml:"iamInstanceProfile,omitempty"`
	// RootVolumeSize in GiB. Default: 20.
	RootVolumeSize int32 `yaml:"rootVolumeSize,omitempty"`
	// K6Version to install. Default: "latest".
	K6Version string `yaml:"k6Version,omitempty"`
	// Env is a map of environment variables to pass to k6.
	Env map[string]string `yaml:"env,omitempty"`
	// Arguments are additional CLI arguments passed to k6 run.
	Arguments []string `yaml:"arguments,omitempty"`
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

// ExecutionSpec defines EC2-specific execution configuration.
type ExecutionSpec struct {
	// Subnets to launch instances into (distributed round-robin).
	Subnets []string `yaml:"subnets"`
	// SecurityGroups for the instances.
	SecurityGroups []string `yaml:"securityGroups,omitempty"`
	// AssignPublicIP controls whether instances get public IPs.
	AssignPublicIP bool `yaml:"assignPublicIP,omitempty"`
	// Region is the AWS region.
	Region string `yaml:"region,omitempty"`
	// Timeout is the maximum duration for the test run.
	Timeout string `yaml:"timeout,omitempty"`
	// SSMEnabled controls whether SSM Run Command is used (defaults to true).
	SSMEnabled *bool `yaml:"ssmEnabled,omitempty"`
	// EIPAllocationIDs are pre-allocated Elastic IP allocation IDs to associate
	// with runner instances. Required for WAF IP-based allowlisting.
	// Length must be >= spec.runner.parallelism when set.
	EIPAllocationIDs []string `yaml:"eipAllocationIDs,omitempty"`
}

// IsSSMEnabled returns whether SSM execution is enabled (defaults to true).
func (e *ExecutionSpec) IsSSMEnabled() bool {
	if e.SSMEnabled == nil {
		return true
	}
	return *e.SSMEnabled
}

// InstanceStatus represents the status of a single EC2 instance.
type InstanceStatus struct {
	InstanceID string `json:"instanceId"`
	PublicIP   string `json:"publicIp,omitempty"`
	PrivateIP  string `json:"privateIp,omitempty"`
	State      string `json:"state"`
	ExitCode   *int   `json:"exitCode,omitempty"`
	RunnerID   int32  `json:"runnerId"`
	SpotID     string `json:"spotId,omitempty"`
}

// Load reads and parses a Config from a YAML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	return Parse(data)
}

// Parse parses a Config from YAML bytes.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	applyDefaults(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	r := &cfg.Runner
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

	if cfg.Execution.Timeout == "" {
		cfg.Execution.Timeout = DefaultTimeout
	}
	if cfg.Cleanup == "" {
		cfg.Cleanup = DefaultCleanup
	}
}

func validate(cfg *Config) error {
	if err := types.ValidateName(cfg.Name); err != nil {
		return err
	}
	if err := types.ValidateScript(&cfg.Script); err != nil {
		return fmt.Errorf("script: %w", err)
	}
	if err := types.ValidateParallelism(cfg.Runner.Parallelism); err != nil {
		return fmt.Errorf("runner: %w", err)
	}

	if len(cfg.Execution.Subnets) == 0 {
		return fmt.Errorf("execution.subnets is required")
	}

	if eips := cfg.Execution.EIPAllocationIDs; len(eips) > 0 {
		if int32(len(eips)) < cfg.Runner.Parallelism {
			return fmt.Errorf("execution.eipAllocationIDs: need at least %d EIPs for parallelism %d, got %d",
				cfg.Runner.Parallelism, cfg.Runner.Parallelism, len(eips))
		}
	}

	if err := types.ValidateTimeout(cfg.Execution.Timeout); err != nil {
		return fmt.Errorf("execution.timeout: %w", err)
	}
	if err := types.ValidateCleanup(cfg.Cleanup); err != nil {
		return fmt.Errorf("cleanup: %w", err)
	}

	return nil
}
