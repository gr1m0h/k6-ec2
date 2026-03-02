package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gr1m0h/k6-ec2/internal/config"
	"github.com/gr1m0h/k6-ec2/pkg/types"
)

func TestPipelineState_InstanceIDs(t *testing.T) {
	tests := []struct {
		name      string
		instances []config.InstanceStatus
		want      []string
	}{
		{
			name:      "empty",
			instances: nil,
			want:      []string{},
		},
		{
			name: "single instance",
			instances: []config.InstanceStatus{
				{InstanceID: "i-abc123"},
			},
			want: []string{"i-abc123"},
		},
		{
			name: "multiple instances",
			instances: []config.InstanceStatus{
				{InstanceID: "i-abc123", RunnerID: 0},
				{InstanceID: "i-def456", RunnerID: 1},
				{InstanceID: "i-ghi789", RunnerID: 2},
			},
			want: []string{"i-abc123", "i-def456", "i-ghi789"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &PipelineState{Instances: tt.instances}
			got := s.InstanceIDs()
			if len(got) != len(tt.want) {
				t.Fatalf("InstanceIDs() returned %d items, want %d", len(got), len(tt.want))
			}
			for i, id := range got {
				if id != tt.want[i] {
					t.Errorf("InstanceIDs()[%d] = %q, want %q", i, id, tt.want[i])
				}
			}
		})
	}
}

func TestSaveAndLoadState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	original := &PipelineState{
		TestName: "my-test",
		Region:   "ap-northeast-1",
		ScriptS3: &types.S3Location{
			Bucket: "test-bucket",
			Key:    "scripts/test.js",
		},
		AMI: "ami-0123456789abcdef0",
		Instances: []config.InstanceStatus{
			{InstanceID: "i-abc123", RunnerID: 0, State: "running"},
			{InstanceID: "i-def456", RunnerID: 1, State: "running"},
		},
		SpotCount:     1,
		FallbackCount: 1,
		LogGroup:      "/k6-ec2/my-test",
		Phase:         "launched",
		CreatedAt:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := SaveState(path, original); err != nil {
		t.Fatalf("SaveState() error: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}

	// Verify fields
	if loaded.TestName != original.TestName {
		t.Errorf("TestName = %q, want %q", loaded.TestName, original.TestName)
	}
	if loaded.Region != original.Region {
		t.Errorf("Region = %q, want %q", loaded.Region, original.Region)
	}
	if loaded.AMI != original.AMI {
		t.Errorf("AMI = %q, want %q", loaded.AMI, original.AMI)
	}
	if loaded.Phase != original.Phase {
		t.Errorf("Phase = %q, want %q", loaded.Phase, original.Phase)
	}
	if loaded.LogGroup != original.LogGroup {
		t.Errorf("LogGroup = %q, want %q", loaded.LogGroup, original.LogGroup)
	}
	if loaded.SpotCount != original.SpotCount {
		t.Errorf("SpotCount = %d, want %d", loaded.SpotCount, original.SpotCount)
	}
	if loaded.FallbackCount != original.FallbackCount {
		t.Errorf("FallbackCount = %d, want %d", loaded.FallbackCount, original.FallbackCount)
	}
	if loaded.ScriptS3 == nil {
		t.Fatal("ScriptS3 is nil")
	}
	if loaded.ScriptS3.Bucket != "test-bucket" || loaded.ScriptS3.Key != "scripts/test.js" {
		t.Errorf("ScriptS3 = %+v, want bucket=test-bucket key=scripts/test.js", loaded.ScriptS3)
	}
	if len(loaded.Instances) != 2 {
		t.Fatalf("len(Instances) = %d, want 2", len(loaded.Instances))
	}
	if loaded.Instances[0].InstanceID != "i-abc123" {
		t.Errorf("Instances[0].InstanceID = %q, want i-abc123", loaded.Instances[0].InstanceID)
	}
	if loaded.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set by SaveState")
	}
}

func TestSaveState_SetsUpdatedAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	state := &PipelineState{
		TestName: "test",
		Phase:    "prepared",
	}

	before := time.Now()
	if err := SaveState(path, state); err != nil {
		t.Fatalf("SaveState() error: %v", err)
	}

	if state.UpdatedAt.Before(before) {
		t.Error("UpdatedAt should be set to current time")
	}
}

func TestLoadState_FileNotFound(t *testing.T) {
	_, err := LoadState("/nonexistent/path/state.json")
	if err == nil {
		t.Fatal("LoadState() should return error for nonexistent file")
	}
}

func TestLoadState_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadState(path)
	if err == nil {
		t.Fatal("LoadState() should return error for invalid JSON")
	}
}

func TestPipelineState_JSONRoundTrip(t *testing.T) {
	state := &PipelineState{
		TestName: "roundtrip-test",
		Region:   "us-west-2",
		Phase:    "executed",
		Instances: []config.InstanceStatus{
			{InstanceID: "i-111", RunnerID: 0},
		},
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded PipelineState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.TestName != state.TestName {
		t.Errorf("TestName = %q, want %q", decoded.TestName, state.TestName)
	}
	if decoded.Phase != state.Phase {
		t.Errorf("Phase = %q, want %q", decoded.Phase, state.Phase)
	}
	if len(decoded.Instances) != 1 || decoded.Instances[0].InstanceID != "i-111" {
		t.Errorf("Instances roundtrip failed: %+v", decoded.Instances)
	}
}
