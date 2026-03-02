package result

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gr1m0h/k6-ec2/pkg/types"
)

// Summary holds the result summary of a test run.
type Summary struct {
	Name        string               `json:"name"`
	Platform    string               `json:"platform"`
	Phase       string               `json:"phase"`
	Parallelism int32                `json:"parallelism"`
	StartTime   *time.Time           `json:"startTime,omitempty"`
	EndTime     *time.Time           `json:"endTime,omitempty"`
	Duration    string               `json:"duration,omitempty"`
	Results     []types.RunnerResult `json:"results"`
	Spot        *types.SpotInfo      `json:"spot,omitempty"`
}

// NewSummary creates a new result Summary.
func NewSummary(
	name, platform, phase string,
	parallelism int32,
	start, end *time.Time,
	results []types.RunnerResult,
	spot *types.SpotInfo,
) *Summary {
	s := &Summary{
		Name:        name,
		Platform:    platform,
		Phase:       phase,
		Parallelism: parallelism,
		StartTime:   start,
		EndTime:     end,
		Results:     results,
		Spot:        spot,
	}
	if start != nil && end != nil {
		s.Duration = end.Sub(*start).Round(time.Second).String()
	}
	return s
}

// FormatJSON returns the summary as a formatted JSON string.
func (s *Summary) FormatJSON() (string, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatText returns the summary as a human-readable text string.
func (s *Summary) FormatText() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n=== Test Run: %s ===\n", s.Name))
	sb.WriteString(fmt.Sprintf("Platform:    %s\n", s.Platform))
	sb.WriteString(fmt.Sprintf("Phase:       %s\n", s.Phase))
	sb.WriteString(fmt.Sprintf("Parallelism: %d\n", s.Parallelism))
	if s.Duration != "" {
		sb.WriteString(fmt.Sprintf("Duration:    %s\n", s.Duration))
	}
	if s.Spot != nil {
		sb.WriteString(fmt.Sprintf("Spot:        %d (fallback: %d)\n", s.Spot.Count, s.Spot.Fallback))
	}
	sb.WriteString("\nRunners:\n")
	for _, r := range s.Results {
		exitStr := "n/a"
		if r.ExitCode != nil {
			exitStr = fmt.Sprintf("%d", *r.ExitCode)
		}
		sb.WriteString(fmt.Sprintf("  %-12s %-10s exit=%s\n", r.Label, r.Status, exitStr))
	}
	return sb.String()
}
