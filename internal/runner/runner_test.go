package runner

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/gr1m0h/k6-ec2/internal/config"
	"github.com/gr1m0h/k6-ec2/pkg/types"
)

// newTestRunner creates a Runner for testing without AWS clients.
func newTestRunner(spec *config.Config) *Runner {
	return &Runner{
		spec:   spec,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		phase:  types.PhaseCompleted,
	}
}

func TestEvaluateResults(t *testing.T) {
	tests := []struct {
		name      string
		instances []config.InstanceStatus
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "no instances",
			instances: nil,
			wantErr:   false,
		},
		{
			name: "all instances exit 0",
			instances: []config.InstanceStatus{
				{InstanceID: "i-abc123", ExitCode: intPtr(0)},
				{InstanceID: "i-def456", ExitCode: intPtr(0)},
				{InstanceID: "i-ghi789", ExitCode: intPtr(0)},
			},
			wantErr: false,
		},
		{
			name: "one instance exit 1",
			instances: []config.InstanceStatus{
				{InstanceID: "i-abc123", ExitCode: intPtr(0)},
				{InstanceID: "i-def456", ExitCode: intPtr(1)},
			},
			wantErr: true,
			errMsg:  "1 instance(s) failed: instance i-def456 exited with code 1",
		},
		{
			name: "multiple failed instances",
			instances: []config.InstanceStatus{
				{InstanceID: "i-abc123", ExitCode: intPtr(1)},
				{InstanceID: "i-def456", ExitCode: intPtr(2)},
				{InstanceID: "i-ghi789", ExitCode: intPtr(0)},
			},
			wantErr: true,
			errMsg:  "2 instance(s) failed",
		},
		{
			name: "nil exit codes",
			instances: []config.InstanceStatus{
				{InstanceID: "i-abc123", ExitCode: nil},
				{InstanceID: "i-def456", ExitCode: nil},
			},
			wantErr: false,
		},
		{
			name: "mixed exit codes",
			instances: []config.InstanceStatus{
				{InstanceID: "i-abc123", ExitCode: intPtr(0)},
				{InstanceID: "i-def456", ExitCode: nil},
				{InstanceID: "i-ghi789", ExitCode: intPtr(127)},
			},
			wantErr: true,
			errMsg:  "instance i-ghi789 exited with code 127",
		},
		{
			name: "exit code 255",
			instances: []config.InstanceStatus{
				{InstanceID: "i-abc123", ExitCode: intPtr(255)},
			},
			wantErr: true,
			errMsg:  "instance i-abc123 exited with code 255",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRunner(&config.Config{Name: "test"})
			r.instances = tt.instances

			err := r.EvaluateResults()
			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateResults() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("EvaluateResults() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestSummary(t *testing.T) {
	start := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 1, 12, 30, 0, 0, time.UTC)

	tests := []struct {
		name          string
		spec          *config.Config
		instances     []config.InstanceStatus
		spotCount     int
		fallbackCount int
		startTime     *time.Time
		endTime       *time.Time
		phase         types.TestRunPhase
		wantSpot      bool
	}{
		{
			name: "basic summary with instances",
			spec: &config.Config{
				Name: "basic-test",
				Runner: config.RunnerSpec{
					InstanceType: "c5.xlarge",
					Parallelism:  2,
				},
			},
			instances: []config.InstanceStatus{
				{InstanceID: "i-abc123", RunnerID: 0, State: "running", ExitCode: intPtr(0)},
				{InstanceID: "i-def456", RunnerID: 1, State: "running", ExitCode: intPtr(0)},
			},
			startTime: &start,
			endTime:   &end,
			phase:     types.PhaseCompleted,
			wantSpot:  false,
		},
		{
			name: "with spot enabled",
			spec: &config.Config{
				Name: "spot-test",
				Runner: config.RunnerSpec{
					InstanceType: "c5.xlarge",
					Parallelism:  3,
					Spot: config.SpotConfig{
						Enabled: true,
					},
				},
			},
			instances: []config.InstanceStatus{
				{InstanceID: "i-abc123", RunnerID: 0, State: "running", ExitCode: intPtr(0)},
				{InstanceID: "i-def456", RunnerID: 1, State: "running", ExitCode: intPtr(0)},
				{InstanceID: "i-ghi789", RunnerID: 2, State: "running", ExitCode: intPtr(1)},
			},
			spotCount:     2,
			fallbackCount: 1,
			startTime:     &start,
			endTime:       &end,
			phase:         types.PhaseCompleted,
			wantSpot:      true,
		},
		{
			name: "spot disabled - no spot info",
			spec: &config.Config{
				Name: "ondemand-test",
				Runner: config.RunnerSpec{
					InstanceType: "c5.xlarge",
					Parallelism:  2,
					Spot: config.SpotConfig{
						Enabled: false,
					},
				},
			},
			instances: []config.InstanceStatus{
				{InstanceID: "i-abc123", RunnerID: 0, State: "running"},
			},
			spotCount:     0,
			fallbackCount: 0,
			startTime:     &start,
			endTime:       &end,
			phase:         types.PhaseCompleted,
			wantSpot:      false,
		},
		{
			name: "no instances - empty results",
			spec: &config.Config{
				Name: "empty-test",
				Runner: config.RunnerSpec{
					InstanceType: "c5.xlarge",
					Parallelism:  0,
				},
			},
			instances: nil,
			startTime: &start,
			endTime:   &end,
			phase:     types.PhaseInitializing,
			wantSpot:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRunner(tt.spec)
			r.instances = tt.instances
			r.spotCount = tt.spotCount
			r.fallbackCount = tt.fallbackCount
			r.startTime = tt.startTime
			r.endTime = tt.endTime
			r.phase = tt.phase

			summary := r.Summary()
			if summary == nil {
				t.Fatal("Summary() returned nil")
			}

			if summary.Name != tt.spec.Name {
				t.Errorf("Summary.Name = %q, want %q", summary.Name, tt.spec.Name)
			}

			expectedPlatform := "EC2/" + tt.spec.Runner.InstanceType
			if summary.Platform != expectedPlatform {
				t.Errorf("Summary.Platform = %q, want %q", summary.Platform, expectedPlatform)
			}

			if summary.Phase != string(tt.phase) {
				t.Errorf("Summary.Phase = %q, want %q", summary.Phase, string(tt.phase))
			}

			if summary.Parallelism != tt.spec.Runner.Parallelism {
				t.Errorf("Summary.Parallelism = %d, want %d", summary.Parallelism, tt.spec.Runner.Parallelism)
			}

			if len(summary.Results) != len(tt.instances) {
				t.Errorf("Summary.Results length = %d, want %d", len(summary.Results), len(tt.instances))
			}

			for i, result := range summary.Results {
				expectedID := tt.instances[i].InstanceID
				if result.ID != expectedID {
					t.Errorf("Summary.Results[%d].ID = %q, want %q", i, result.ID, expectedID)
				}

				if !strings.Contains(result.Label, "runner-") {
					t.Errorf("Summary.Results[%d].Label = %q, want format 'runner-N'", i, result.Label)
				}
			}

			if tt.wantSpot {
				if summary.Spot == nil {
					t.Error("Summary.Spot = nil, want non-nil")
				} else {
					if summary.Spot.Count != tt.spotCount {
						t.Errorf("Summary.Spot.Count = %d, want %d", summary.Spot.Count, tt.spotCount)
					}
					if summary.Spot.Fallback != tt.fallbackCount {
						t.Errorf("Summary.Spot.Fallback = %d, want %d", summary.Spot.Fallback, tt.fallbackCount)
					}
				}
			} else {
				if summary.Spot != nil {
					t.Error("Summary.Spot = non-nil, want nil (spot not enabled)")
				}
			}
		})
	}
}

func TestLogGroup(t *testing.T) {
	tests := []struct {
		name     string
		testName string
		want     string
	}{
		{
			name:     "simple name",
			testName: "my-test",
			want:     "/k6-ec2/my-test",
		},
		{
			name:     "name with hyphen",
			testName: "load-test-prod",
			want:     "/k6-ec2/load-test-prod",
		},
		{
			name:     "single character",
			testName: "a",
			want:     "/k6-ec2/a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRunner(&config.Config{Name: tt.testName})
			got := r.LogGroup()
			if got != tt.want {
				t.Errorf("LogGroup() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRegion(t *testing.T) {
	tests := []struct {
		name   string
		region string
	}{
		{
			name:   "us-east-1",
			region: "us-east-1",
		},
		{
			name:   "ap-northeast-1",
			region: "ap-northeast-1",
		},
		{
			name:   "eu-west-1",
			region: "eu-west-1",
		},
		{
			name:   "empty region",
			region: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRunner(&config.Config{
				Execution: config.ExecutionSpec{
					Region: tt.region,
				},
			})
			got := r.Region()
			if got != tt.region {
				t.Errorf("Region() = %q, want %q", got, tt.region)
			}
		})
	}
}

func TestInstanceIDs(t *testing.T) {
	tests := []struct {
		name      string
		instances []config.InstanceStatus
		want      []string
	}{
		{
			name:      "empty instances",
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
			r := newTestRunner(&config.Config{Name: "test"})
			r.instances = tt.instances

			got := r.instanceIDs()
			if len(got) != len(tt.want) {
				t.Fatalf("instanceIDs() returned %d items, want %d", len(got), len(tt.want))
			}
			for i, id := range got {
				if id != tt.want[i] {
					t.Errorf("instanceIDs()[%d] = %q, want %q", i, id, tt.want[i])
				}
			}
		})
	}
}

func TestBuildUserData(t *testing.T) {
	tests := []struct {
		name           string
		spec           *config.Config
		runnerID       int32
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "basic user data with latest k6",
			spec: &config.Config{
				Runner: config.RunnerSpec{
					K6Version:   "latest",
					Parallelism: 3,
				},
			},
			runnerID: 0,
			wantContains: []string{
				"#!/bin/bash",
				"set -euo pipefail",
				"export K6_INSTANCE_ID=0",
				"export K6_PARALLELISM=3",
				"# Install k6",
				"curl -sL https://github.com/grafana/k6/releases/latest/download/k6-linux-amd64.tar.gz",
				"mv k6-*/k6 /usr/local/bin/k6",
				"chmod +x /usr/local/bin/k6",
			},
		},
		{
			name: "specific k6 version",
			spec: &config.Config{
				Runner: config.RunnerSpec{
					K6Version:   "0.45.0",
					Parallelism: 1,
				},
			},
			runnerID: 0,
			wantContains: []string{
				"curl -sL https://github.com/grafana/k6/releases/download/v0.45.0/k6-v0.45.0-linux-amd64.tar.gz",
			},
		},
		{
			name: "with environment variables",
			spec: &config.Config{
				Runner: config.RunnerSpec{
					K6Version:   "latest",
					Parallelism: 2,
					Env: map[string]string{
						"API_KEY":  "secret123",
						"BASE_URL": "https://example.com",
					},
				},
			},
			runnerID: 1,
			wantContains: []string{
				"export K6_INSTANCE_ID=1",
				"export K6_PARALLELISM=2",
				`export API_KEY="secret123"`,
				`export BASE_URL="https://example.com"`,
			},
		},
		{
			name: "with extra user data",
			spec: &config.Config{
				Runner: config.RunnerSpec{
					K6Version:     "latest",
					Parallelism:   1,
					UserDataExtra: "echo 'Custom setup'\nyum install -y jq",
				},
			},
			runnerID: 0,
			wantContains: []string{
				"# Extra user data",
				"echo 'Custom setup'",
				"yum install -y jq",
			},
		},
		{
			name: "with output spec - statsd",
			spec: &config.Config{
				Runner: config.RunnerSpec{
					K6Version:   "latest",
					Parallelism: 2,
				},
				Output: types.OutputSpec{
					StatSD: &types.StatsDSpec{
						Address:     "localhost:8125",
						EnabledTags: true,
						Namespace:   "k6",
					},
				},
			},
			runnerID: 0,
			wantContains: []string{
				`export K6_STATSD_ADDR="localhost:8125"`,
				`export K6_STATSD_ENABLE_TAGS="true"`,
				`export K6_STATSD_NAMESPACE="k6"`,
			},
		},
		{
			name: "different runner IDs",
			spec: &config.Config{
				Runner: config.RunnerSpec{
					K6Version:   "latest",
					Parallelism: 5,
				},
			},
			runnerID: 3,
			wantContains: []string{
				"export K6_INSTANCE_ID=3",
				"export K6_PARALLELISM=5",
			},
		},
		{
			name: "no extra user data",
			spec: &config.Config{
				Runner: config.RunnerSpec{
					K6Version:     "latest",
					Parallelism:   1,
					UserDataExtra: "",
				},
			},
			runnerID: 0,
			wantNotContain: []string{
				"# Extra user data",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRunner(tt.spec)
			got := r.buildUserData(tt.runnerID)

			if !strings.HasPrefix(got, "#!/bin/bash\n") {
				t.Error("buildUserData() should start with shebang")
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("buildUserData() missing expected content: %q", want)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(got, notWant) {
					t.Errorf("buildUserData() contains unexpected content: %q", notWant)
				}
			}
		})
	}
}

func TestBuildUserData_OutputEnvVars(t *testing.T) {
	// Test that output env vars are generated correctly
	r := newTestRunner(&config.Config{
		Runner: config.RunnerSpec{
			K6Version:   "latest",
			Parallelism: 1,
		},
		Output: types.OutputSpec{
			StatSD: &types.StatsDSpec{
				Address: "127.0.0.1:8125",
			},
		},
	})

	userData := r.buildUserData(0)

	if !strings.Contains(userData, "K6_STATSD_ADDR") {
		t.Error("buildUserData() should include K6_STATSD_ADDR when StatSD output is configured")
	}
}

func TestBuildUserData_EnvVarQuoting(t *testing.T) {
	// Test that env var values are properly quoted
	r := newTestRunner(&config.Config{
		Runner: config.RunnerSpec{
			K6Version:   "latest",
			Parallelism: 1,
			Env: map[string]string{
				"VAR_WITH_SPACES": "value with spaces",
				"VAR_WITH_QUOTES": `value"with"quotes`,
			},
		},
	})

	userData := r.buildUserData(0)

	// Should use double quotes for env var values
	if !strings.Contains(userData, `export VAR_WITH_SPACES="value with spaces"`) {
		t.Error("buildUserData() should properly quote env var values with spaces")
	}
}

// intPtr returns a pointer to an int.
func intPtr(i int) *int {
	return &i
}
