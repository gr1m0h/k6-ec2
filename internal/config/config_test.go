package config

import "testing"

func TestParse_ValidConfig(t *testing.T) {
	yaml := `
apiVersion: k6-ec2.io/v1alpha1
kind: EC2TestRun
metadata:
  name: test
spec:
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
	spec, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Metadata.Name != "test" {
		t.Errorf("expected name 'test', got %q", spec.Metadata.Name)
	}
	if spec.Spec.Runner.Parallelism != 4 {
		t.Errorf("expected parallelism 4, got %d", spec.Spec.Runner.Parallelism)
	}
	if !spec.Spec.Runner.Spot.Enabled {
		t.Error("expected spot enabled")
	}
}

func TestParse_AppliesDefaults(t *testing.T) {
	yaml := `
metadata:
  name: test
spec:
  script:
    inline: "test"
  execution:
    subnets: [subnet-abc]
`
	spec, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Kind != "EC2TestRun" {
		t.Errorf("expected Kind EC2TestRun, got %q", spec.Kind)
	}
	if spec.Spec.Runner.InstanceType != DefaultInstanceType {
		t.Errorf("expected instance type %q, got %q", DefaultInstanceType, spec.Spec.Runner.InstanceType)
	}
	if spec.Spec.Runner.K6Version != DefaultK6Version {
		t.Errorf("expected k6 version %q, got %q", DefaultK6Version, spec.Spec.Runner.K6Version)
	}
	if spec.Spec.Runner.RootVolumeSize != int32(DefaultRootVolume) {
		t.Errorf("expected root volume %d, got %d", DefaultRootVolume, spec.Spec.Runner.RootVolumeSize)
	}
	if spec.Spec.Cleanup.Policy != DefaultCleanup {
		t.Errorf("expected cleanup %q, got %q", DefaultCleanup, spec.Spec.Cleanup.Policy)
	}
	if !spec.Spec.Execution.IsSSMEnabled() {
		t.Error("expected SSM enabled by default")
	}
}

func TestParse_MissingName(t *testing.T) {
	yaml := `
spec:
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
metadata:
  name: test
spec:
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
metadata:
  name: test
spec:
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
metadata:
  name: test
spec:
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
metadata:
  name: test
spec:
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
	spec, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spec.Spec.Execution.EIPAllocationIDs) != 2 {
		t.Errorf("expected 2 EIPs, got %d", len(spec.Spec.Execution.EIPAllocationIDs))
	}
}

func TestParse_EIPAllocationIDs_Empty(t *testing.T) {
	yaml := `
metadata:
  name: test
spec:
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
