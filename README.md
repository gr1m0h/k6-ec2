# k6-ec2

![](./images/logo.svg)

A CLI tool that runs k6 load tests on EC2 instances as seamlessly as running k6 locally. It abstracts away SSM connections and automates the entire workflow — from script upload to instance cleanup.

## Architecture

![](./images/architecture.svg)

## Features

- **SSM Run Command** — No SSH required, integrated with CloudWatch Logs
- **Distributed Execution** — Run k6 in parallel across multiple EC2 instances (just set `parallelism`)
- **Spot Instances** — Automatic fallback to on-demand
- **Terraform Module** — One-step infrastructure setup (S3, IAM, SG, CloudWatch)
- **Declarative YAML** — Define test runs with a simple config file

## Quick Start

```bash
# 1. Install
go install github.com/gr1m0h/k6-ec2/cmd/k6-ec2@latest

# 2. Set up infrastructure
cd terraform/examples
terraform init && terraform apply \
  -var="vpc_id=vpc-xxx" \
  -var='subnet_ids=["subnet-aaa"]'

# 3. Run
k6-ec2 init -o testrun.yaml   # Generate a sample config
k6-ec2 run -f testrun.yaml    # Execute load test
```

## Configuration

```yaml
name: my-load-test

script:
  localFile: ./scripts/test.js

runner:
  instanceType: c5.xlarge
  parallelism: 4
  iamInstanceProfile: k6-ec2-runner
  spot:
    enabled: true
    fallbackToOnDemand: true

execution:
  subnets: [subnet-xxx]
  securityGroups: [sg-xxx]
  assignPublicIP: true
  region: ap-northeast-1
  timeout: 30m
  ssmEnabled: true

cleanup: always  # always | on-success | never
```

## CLI

| Command | Description |
| :--- | :--- |
| `k6-ec2 run -f config.yaml` | Run load test (prepare → launch → execute → cleanup) |
| `k6-ec2 validate -f config.yaml` | Validate config file |
| `k6-ec2 logs --test-name X -f` | Stream CloudWatch Logs |
| `k6-ec2 init -o config.yaml` | Generate a sample config file |
| `k6-ec2 version` | Show version |

## Terraform Module

The module in `terraform/modules/` creates the following resources:

| Resource | Purpose |
| :--- | :--- |
| S3 Bucket | Store k6 test scripts |
| IAM Role / Instance Profile | For EC2 runner instances |
| IAM Policy (CLI) | For CLI operator |
| Security Group | For runner instances |
| CloudWatch Log Group | SSM execution logs |
| Elastic IP (optional) | For WAF IP-based allow lists |
