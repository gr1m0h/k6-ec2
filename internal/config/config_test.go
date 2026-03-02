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
