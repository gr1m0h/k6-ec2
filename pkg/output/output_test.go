package output

import (
	"testing"

	"github.com/gr1m0h/k6-ec2/pkg/types"
)

func TestBuild_NilSpec(t *testing.T) {
	envs, err := Build(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 0 {
		t.Errorf("expected 0 envs, got %d", len(envs))
	}
}

func TestBuild_NilStatSD(t *testing.T) {
	spec := &types.OutputSpec{}
	envs, err := Build(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 0 {
		t.Errorf("expected 0 envs, got %d", len(envs))
	}
}

func TestBuild_FullStatSD(t *testing.T) {
	spec := &types.OutputSpec{
		StatSD: &types.StatsDSpec{
			Address:     "localhost:8125",
			EnabledTags: true,
			Namespace:   "k6.",
		},
	}
	envs, err := Build(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 3 {
		t.Fatalf("expected 3 envs, got %d", len(envs))
	}
	assertEnv(t, envs, "K6_STATSD_ADDR", "localhost:8125")
	assertEnv(t, envs, "K6_STATSD_ENABLE_TAGS", "true")
	assertEnv(t, envs, "K6_STATSD_NAMESPACE", "k6.")
}

func TestBuild_AddressOnly(t *testing.T) {
	spec := &types.OutputSpec{
		StatSD: &types.StatsDSpec{
			Address: "localhost:8125",
		},
	}
	envs, err := Build(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 1 {
		t.Fatalf("expected 1 env, got %d", len(envs))
	}
	assertEnv(t, envs, "K6_STATSD_ADDR", "localhost:8125")
}

func TestBuild_TagsWithoutAddress(t *testing.T) {
	spec := &types.OutputSpec{
		StatSD: &types.StatsDSpec{
			EnabledTags: true,
		},
	}
	envs, err := Build(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 1 {
		t.Fatalf("expected 1 env, got %d", len(envs))
	}
	assertEnv(t, envs, "K6_STATSD_ENABLE_TAGS", "true")
}

func TestBuildK6Command_NoOutput(t *testing.T) {
	cmd := BuildK6Command(nil, "/tmp/test.js", nil)
	expected := "k6 run /tmp/test.js"
	if cmd != expected {
		t.Errorf("expected %q, got %q", expected, cmd)
	}
}

func TestBuildK6Command_WithStatSD(t *testing.T) {
	spec := &types.OutputSpec{
		StatSD: &types.StatsDSpec{Address: "localhost:8125"},
	}
	cmd := BuildK6Command(spec, "/tmp/test.js", nil)
	expected := "k6 run --out statsd /tmp/test.js"
	if cmd != expected {
		t.Errorf("expected %q, got %q", expected, cmd)
	}
}

func TestBuildK6Command_WithArgs(t *testing.T) {
	cmd := BuildK6Command(nil, "/tmp/test.js", []string{"--vus", "10", "--duration", "30s"})
	expected := "k6 run --vus 10 --duration 30s /tmp/test.js"
	if cmd != expected {
		t.Errorf("expected %q, got %q", expected, cmd)
	}
}

func TestBuildK6Command_WithStatSDAndArgs(t *testing.T) {
	spec := &types.OutputSpec{
		StatSD: &types.StatsDSpec{Address: "localhost:8125"},
	}
	cmd := BuildK6Command(spec, "/tmp/test.js", []string{"--vus", "10"})
	expected := "k6 run --out statsd --vus 10 /tmp/test.js"
	if cmd != expected {
		t.Errorf("expected %q, got %q", expected, cmd)
	}
}

func TestBuildK6Command_EmptyStatSD(t *testing.T) {
	spec := &types.OutputSpec{
		StatSD: &types.StatsDSpec{},
	}
	cmd := BuildK6Command(spec, "/tmp/test.js", nil)
	expected := "k6 run /tmp/test.js"
	if cmd != expected {
		t.Errorf("expected %q (no --out when address empty), got %q", expected, cmd)
	}
}

func assertEnv(t *testing.T, envs []EnvVar, key, value string) {
	t.Helper()
	for _, e := range envs {
		if e.Key == key {
			if e.Value != value {
				t.Errorf("env %s: expected %q, got %q", key, value, e.Value)
			}
			return
		}
	}
	t.Errorf("env %s not found", key)
}
