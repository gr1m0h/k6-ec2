package config

import "testing"

func TestParse_ValidConfig(t *testing.T) {
	yaml := `
name: test
script:
  inline: |
    import http from 'k6/http';
    export default function() { http.get('https://test.k6.io'); }
runner:
  parallelism: 4
  instanceType: c5.xlarge
  spot:
    enabled: true
    fallbackToOnDemand: true
execution:
  subnets:
    - subnet-abc
  region: ap-northeast-1
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "test" {
		t.Errorf("expected name 'test', got %q", cfg.Name)
	}
	if cfg.Runner.Parallelism != 4 {
		t.Errorf("expected parallelism 4, got %d", cfg.Runner.Parallelism)
	}
	if !cfg.Runner.Spot.Enabled {
		t.Error("expected spot enabled")
	}
}

func TestParse_AppliesDefaults(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
execution:
  subnets: [subnet-abc]
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Runner.InstanceType != DefaultInstanceType {
		t.Errorf("expected instance type %q, got %q", DefaultInstanceType, cfg.Runner.InstanceType)
	}
	if cfg.Runner.K6Version != DefaultK6Version {
		t.Errorf("expected k6 version %q, got %q", DefaultK6Version, cfg.Runner.K6Version)
	}
	if cfg.Runner.RootVolumeSize != int32(DefaultRootVolume) {
		t.Errorf("expected root volume %d, got %d", DefaultRootVolume, cfg.Runner.RootVolumeSize)
	}
	if cfg.Cleanup != DefaultCleanup {
		t.Errorf("expected cleanup %q, got %q", DefaultCleanup, cfg.Cleanup)
	}
}

func TestParse_MissingName(t *testing.T) {
	yaml := `
script:
  inline: "test"
execution:
  subnets: [subnet-abc]
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParse_MissingSubnets(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
execution:
  region: ap-northeast-1
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing subnets")
	}
}

func TestParse_InvalidParallelism(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
runner:
  parallelism: 101
execution:
  subnets: [subnet-abc]
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for parallelism > 100")
	}
}

func TestParse_EIPAllocationIDs_InsufficientCount(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
runner:
  parallelism: 4
execution:
  subnets: [subnet-abc]
  eipAllocationIDs:
    - eipalloc-aaa
    - eipalloc-bbb
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for insufficient EIP count")
	}
}

func TestParse_EIPAllocationIDs_Valid(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
runner:
  parallelism: 2
execution:
  subnets: [subnet-abc]
  eipAllocationIDs:
    - eipalloc-aaa
    - eipalloc-bbb
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Execution.EIPAllocationIDs) != 2 {
		t.Errorf("expected 2 EIPs, got %d", len(cfg.Execution.EIPAllocationIDs))
	}
}

func TestParse_EIPAllocationIDs_Empty(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
runner:
  parallelism: 4
execution:
  subnets: [subnet-abc]
`
	_, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseForCommand_Launch_RequiresSubnets(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
runner:
  parallelism: 2
`
	_, err := ParseForCommand([]byte(yaml), CommandLaunch, nil)
	if err == nil {
		t.Fatal("expected error for missing subnets in launch")
	}
}

func TestParseForCommand_Execute_NoSubnetsOK(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
`
	_, err := ParseForCommand([]byte(yaml), CommandExecute, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseForCommand_Overrides(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
runner:
  parallelism: 1
  instanceType: t3.micro
execution:
  subnets: [subnet-abc]
  region: us-east-1
  timeout: 10m
cleanup: always
`
	p := int32(8)
	it := "c5.2xlarge"
	region := "ap-northeast-1"
	timeout := "1h"
	cleanup := "never"
	overrides := &Overrides{
		Parallelism:  &p,
		InstanceType: &it,
		Region:       &region,
		Timeout:      &timeout,
		Cleanup:      &cleanup,
	}
	cfg, err := ParseForCommand([]byte(yaml), CommandRun, overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Runner.Parallelism != 8 {
		t.Errorf("expected parallelism 8, got %d", cfg.Runner.Parallelism)
	}
	if cfg.Runner.InstanceType != "c5.2xlarge" {
		t.Errorf("expected instance type 'c5.2xlarge', got %q", cfg.Runner.InstanceType)
	}
	if cfg.Execution.Region != "ap-northeast-1" {
		t.Errorf("expected region 'ap-northeast-1', got %q", cfg.Execution.Region)
	}
	if cfg.Execution.Timeout != "1h" {
		t.Errorf("expected timeout '1h', got %q", cfg.Execution.Timeout)
	}
	if cfg.Cleanup != "never" {
		t.Errorf("expected cleanup 'never', got %q", cfg.Cleanup)
	}
}

func TestParseForCommand_OverridesValidation(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
execution:
  subnets: [subnet-abc]
`
	p := int32(200)
	overrides := &Overrides{Parallelism: &p}
	_, err := ParseForCommand([]byte(yaml), CommandRun, overrides)
	if err == nil {
		t.Fatal("expected error for parallelism > 100 after override")
	}
}

func TestParseForCommand_Cleanup_ValidPolicy(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
cleanup: on-success
`
	cfg, err := ParseForCommand([]byte(yaml), CommandCleanup, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cleanup != "on-success" {
		t.Errorf("expected cleanup 'on-success', got %q", cfg.Cleanup)
	}
}

func TestParseForCommand_Cleanup_InvalidPolicy(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
cleanup: sometimes
`
	_, err := ParseForCommand([]byte(yaml), CommandCleanup, nil)
	if err == nil {
		t.Fatal("expected error for invalid cleanup policy")
	}
}

func TestParseForCommand_Cleanup_NeverPolicy(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
cleanup: never
`
	cfg, err := ParseForCommand([]byte(yaml), CommandCleanup, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cleanup != "never" {
		t.Errorf("expected cleanup 'never', got %q", cfg.Cleanup)
	}
}

func TestParseForCommand_Launch_WithEIPs(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
runner:
  parallelism: 2
execution:
  subnets: [subnet-abc]
  eipAllocationIDs:
    - eipalloc-aaa
    - eipalloc-bbb
`
	cfg, err := ParseForCommand([]byte(yaml), CommandLaunch, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Execution.EIPAllocationIDs) != 2 {
		t.Errorf("expected 2 EIPs, got %d", len(cfg.Execution.EIPAllocationIDs))
	}
}

func TestParseForCommand_Launch_InsufficientEIPs(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
runner:
  parallelism: 4
execution:
  subnets: [subnet-abc]
  eipAllocationIDs:
    - eipalloc-aaa
`
	_, err := ParseForCommand([]byte(yaml), CommandLaunch, nil)
	if err == nil {
		t.Fatal("expected error for insufficient EIPs in launch")
	}
}

func TestParseForCommand_Launch_MissingName(t *testing.T) {
	yaml := `
script:
  inline: "test"
runner:
  parallelism: 2
execution:
  subnets: [subnet-abc]
`
	_, err := ParseForCommand([]byte(yaml), CommandLaunch, nil)
	if err == nil {
		t.Fatal("expected error for missing name in launch")
	}
}

func TestParseForCommand_Launch_InvalidParallelism(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
runner:
  parallelism: 200
execution:
  subnets: [subnet-abc]
`
	_, err := ParseForCommand([]byte(yaml), CommandLaunch, nil)
	if err == nil {
		t.Fatal("expected error for invalid parallelism in launch")
	}
}

func TestParseForCommand_Execute_MissingName(t *testing.T) {
	yaml := `
script:
  inline: "test"
`
	_, err := ParseForCommand([]byte(yaml), CommandExecute, nil)
	if err == nil {
		t.Fatal("expected error for missing name in execute")
	}
}

func TestParseForCommand_Execute_InvalidTimeout(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
execution:
  timeout: invalid
`
	_, err := ParseForCommand([]byte(yaml), CommandExecute, nil)
	if err == nil {
		t.Fatal("expected error for invalid timeout in execute")
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	_, err := Parse([]byte(":::invalid yaml"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParse_MultipleScriptSources(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
  localFile: ./test.js
execution:
  subnets: [subnet-abc]
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for multiple script sources")
	}
}

func TestParse_InvalidCleanupPolicy(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
execution:
  subnets: [subnet-abc]
cleanup: invalid
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid cleanup policy in full validate")
	}
}

func TestParse_InvalidTimeout(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
execution:
  subnets: [subnet-abc]
  timeout: badvalue
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid timeout in full validate")
	}
}

func TestParseForCommand_NilOverrides(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
execution:
  subnets: [subnet-abc]
`
	cfg, err := ParseForCommand([]byte(yaml), CommandRun, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Runner.Parallelism != 1 {
		t.Errorf("expected default parallelism 1, got %d", cfg.Runner.Parallelism)
	}
}

func TestParseForCommand_PartialOverrides(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
execution:
  subnets: [subnet-abc]
  region: us-east-1
`
	region := "eu-west-1"
	overrides := &Overrides{Region: &region}
	cfg, err := ParseForCommand([]byte(yaml), CommandRun, overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Execution.Region != "eu-west-1" {
		t.Errorf("expected region 'eu-west-1', got %q", cfg.Execution.Region)
	}
	// Parallelism should remain default since not overridden
	if cfg.Runner.Parallelism != 1 {
		t.Errorf("expected default parallelism 1, got %d", cfg.Runner.Parallelism)
	}
}
