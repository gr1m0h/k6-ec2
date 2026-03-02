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

// Command represents a CLI subcommand for per-command validation.
type Command string

const (
	CommandRun      Command = "run"
	CommandLaunch   Command = "launch"
	CommandExecute  Command = "execute"
	CommandCleanup  Command = "cleanup"
	CommandValidate Command = "validate"
)

// Overrides holds CLI flag values that override config file settings.
// nil fields indicate no override was specified.
type Overrides struct {
	Parallelism  *int32
	InstanceType *string
	Region       *string
	Timeout      *string
	Cleanup      *string
}

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
	// EIPAllocationIDs are pre-allocated Elastic IP allocation IDs to associate
	// with runner instances. Required for WAF IP-based allowlisting.
	// Length must be >= spec.runner.parallelism when set.
	EIPAllocationIDs []string `yaml:"eipAllocationIDs,omitempty"`
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

// Load reads and parses a Config from a YAML file with full validation.
func Load(path string) (*Config, error) {
	return LoadForCommand(path, CommandValidate, nil)
}

// Parse parses a Config from YAML bytes with full validation.
func Parse(data []byte) (*Config, error) {
	return ParseForCommand(data, CommandValidate, nil)
}

// LoadForCommand reads and parses a Config with command-specific validation and overrides.
func LoadForCommand(path string, cmd Command, overrides *Overrides) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	return ParseForCommand(data, cmd, overrides)
}

// ParseForCommand parses a Config with command-specific validation and overrides.
func ParseForCommand(data []byte, cmd Command, overrides *Overrides) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	applyDefaults(&cfg)
	applyOverrides(&cfg, overrides)

	if err := validateForCommand(&cfg, cmd); err != nil {
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

func applyOverrides(cfg *Config, overrides *Overrides) {
	if overrides == nil {
		return
	}
	if overrides.Parallelism != nil {
		cfg.Runner.Parallelism = *overrides.Parallelism
	}
	if overrides.InstanceType != nil {
		cfg.Runner.InstanceType = *overrides.InstanceType
	}
	if overrides.Region != nil {
		cfg.Execution.Region = *overrides.Region
	}
	if overrides.Timeout != nil {
		cfg.Execution.Timeout = *overrides.Timeout
	}
	if overrides.Cleanup != nil {
		cfg.Cleanup = *overrides.Cleanup
	}
}

func validateForCommand(cfg *Config, cmd Command) error {
	switch cmd {
	case CommandLaunch:
		return validateLaunch(cfg)
	case CommandExecute:
		return validateExecute(cfg)
	case CommandCleanup:
		return validateCleanup(cfg)
	default:
		return validate(cfg)
	}
}

func validateLaunch(cfg *Config) error {
	if err := types.ValidateName(cfg.Name); err != nil {
		return err
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
	return nil
}

func validateExecute(cfg *Config) error {
	if err := types.ValidateName(cfg.Name); err != nil {
		return err
	}
	if err := types.ValidateScript(&cfg.Script); err != nil {
		return fmt.Errorf("script: %w", err)
	}
	if err := types.ValidateTimeout(cfg.Execution.Timeout); err != nil {
		return fmt.Errorf("execution.timeout: %w", err)
	}
	return nil
}

func validateCleanup(cfg *Config) error {
	if err := types.ValidateCleanup(cfg.Cleanup); err != nil {
		return fmt.Errorf("cleanup: %w", err)
	}
	return nil
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
