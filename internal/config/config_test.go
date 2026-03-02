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
	if !cfg.Execution.IsSSMEnabled() {
		t.Error("expected SSM enabled by default")
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

func TestParseForCommand_Prepare_NoSubnetsOK(t *testing.T) {
	yaml := `
name: test
script:
  inline: "test"
`
	cfg, err := ParseForCommand([]byte(yaml), CommandPrepare, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "test" {
		t.Errorf("expected name 'test', got %q", cfg.Name)
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

func TestIsSSMEnabled_DefaultTrue(t *testing.T) {
	e := ExecutionSpec{}
	if !e.IsSSMEnabled() {
		t.Error("expected SSM enabled by default")
	}
}

func TestIsSSMEnabled_ExplicitFalse(t *testing.T) {
	f := false
	e := ExecutionSpec{SSMEnabled: &f}
	if e.IsSSMEnabled() {
		t.Error("expected SSM disabled")
	}
}
