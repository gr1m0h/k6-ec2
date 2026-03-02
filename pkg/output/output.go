package output

import (
	"strings"

	"github.com/gr1m0h/k6-ec2/pkg/types"
)

// EnvVar represents an environment variable key-value pair.
type EnvVar struct {
	Key   string
	Value string
}

// Build generates environment variables from the output specification.
func Build(spec *types.OutputSpec) ([]EnvVar, error) {
	var envs []EnvVar
	if spec == nil || spec.StatSD == nil {
		return envs, nil
	}
	s := spec.StatSD
	if s.Address != "" {
		envs = append(envs, EnvVar{Key: "K6_STATSD_ADDR", Value: s.Address})
	}
	if s.EnabledTags {
		envs = append(envs, EnvVar{Key: "K6_STATSD_ENABLE_TAGS", Value: "true"})
	}
	if s.Namespace != "" {
		envs = append(envs, EnvVar{Key: "K6_STATSD_NAMESPACE", Value: s.Namespace})
	}
	return envs, nil
}

// BuildK6Command builds the k6 run command string with appropriate output flags.
func BuildK6Command(spec *types.OutputSpec, scriptPath string, args []string) string {
	var parts []string
	parts = append(parts, "k6 run")
	if spec != nil && spec.StatSD != nil && spec.StatSD.Address != "" {
		parts = append(parts, "--out statsd")
	}
	parts = append(parts, args...)
	parts = append(parts, scriptPath)
	return strings.Join(parts, " ")
}
