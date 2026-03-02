package result

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gr1m0h/k6-ec2/pkg/types"
)

func TestNewSummary_WithDuration(t *testing.T) {
	start := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 1, 10, 5, 30, 0, time.UTC)
	results := []types.RunnerResult{
		{ID: "i-abc", Label: "runner-1", Status: "success"},
	}
	s := NewSummary("test", "ec2", "completed", 4, &start, &end, results, nil)

	if s.Name != "test" {
		t.Errorf("expected name 'test', got %q", s.Name)
	}
	if s.Duration != "5m30s" {
		t.Errorf("expected duration '5m30s', got %q", s.Duration)
	}
	if s.Parallelism != 4 {
		t.Errorf("expected parallelism 4, got %d", s.Parallelism)
	}
}

func TestNewSummary_NilTimes(t *testing.T) {
	s := NewSummary("test", "ec2", "running", 1, nil, nil, nil, nil)
	if s.Duration != "" {
		t.Errorf("expected empty duration, got %q", s.Duration)
	}
}

func TestNewSummary_StartOnlyNoDuration(t *testing.T) {
	start := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	s := NewSummary("test", "ec2", "running", 1, &start, nil, nil, nil)
	if s.Duration != "" {
		t.Errorf("expected empty duration when end is nil, got %q", s.Duration)
	}
}

func TestNewSummary_WithSpot(t *testing.T) {
	spot := &types.SpotInfo{Used: true, Count: 3, Fallback: 1}
	s := NewSummary("test", "ec2", "completed", 4, nil, nil, nil, spot)
	if s.Spot == nil {
		t.Fatal("expected spot info")
	}
	if s.Spot.Count != 3 {
		t.Errorf("expected spot count 3, got %d", s.Spot.Count)
	}
	if s.Spot.Fallback != 1 {
		t.Errorf("expected spot fallback 1, got %d", s.Spot.Fallback)
	}
}

func TestFormatJSON(t *testing.T) {
	exitCode := 0
	results := []types.RunnerResult{
		{ID: "i-abc", Label: "runner-1", Status: "success", ExitCode: &exitCode},
	}
	s := NewSummary("test", "ec2", "completed", 1, nil, nil, results, nil)
	jsonStr, err := s.FormatJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if parsed["name"] != "test" {
		t.Errorf("expected name 'test' in JSON, got %v", parsed["name"])
	}
	if parsed["platform"] != "ec2" {
		t.Errorf("expected platform 'ec2' in JSON, got %v", parsed["platform"])
	}
}

func TestFormatText_Basic(t *testing.T) {
	exitCode := 0
	results := []types.RunnerResult{
		{ID: "i-abc", Label: "runner-1", Status: "success", ExitCode: &exitCode},
	}
	s := NewSummary("my-test", "ec2", "completed", 2, nil, nil, results, nil)
	text := s.FormatText()

	checks := []string{
		"=== Test Run: my-test ===",
		"Platform:    ec2",
		"Phase:       completed",
		"Parallelism: 2",
		"runner-1",
		"success",
		"exit=0",
	}
	for _, check := range checks {
		if !strings.Contains(text, check) {
			t.Errorf("FormatText() missing %q in output:\n%s", check, text)
		}
	}
}

func TestFormatText_WithDuration(t *testing.T) {
	start := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 1, 10, 2, 0, 0, time.UTC)
	s := NewSummary("test", "ec2", "completed", 1, &start, &end, nil, nil)
	text := s.FormatText()
	if !strings.Contains(text, "Duration:    2m0s") {
		t.Errorf("FormatText() missing duration in output:\n%s", text)
	}
}

func TestFormatText_WithSpot(t *testing.T) {
	spot := &types.SpotInfo{Used: true, Count: 3, Fallback: 1}
	s := NewSummary("test", "ec2", "completed", 4, nil, nil, nil, spot)
	text := s.FormatText()
	if !strings.Contains(text, "Spot:        3 (fallback: 1)") {
		t.Errorf("FormatText() missing spot info in output:\n%s", text)
	}
}

func TestFormatText_NilExitCode(t *testing.T) {
	results := []types.RunnerResult{
		{ID: "i-abc", Label: "runner-1", Status: "pending", ExitCode: nil},
	}
	s := NewSummary("test", "ec2", "running", 1, nil, nil, results, nil)
	text := s.FormatText()
	if !strings.Contains(text, "exit=n/a") {
		t.Errorf("FormatText() expected 'exit=n/a' for nil exit code:\n%s", text)
	}
}
