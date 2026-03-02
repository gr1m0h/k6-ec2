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
labels:
  team: platform
  env: staging

script:
  localFile: ./scripts/test.js
  # s3: s3://bucket/test.js       # Use a pre-uploaded S3 object
  # inline: |                      # Or embed the script inline
  #   import http from 'k6/http';
  #   export default function() { http.get('https://test.k6.io'); }

runner:
  instanceType: c5.xlarge
  parallelism: 4
  iamInstanceProfile: k6-ec2-runner
  # ami: ami-xxxxxxxxxxxxxxxxx     # Default: latest Amazon Linux 2023
  # k6Version: latest              # Default: latest
  # rootVolumeSize: 20             # Default: 20 (GiB)
  spot:
    enabled: true
    fallbackToOnDemand: true
    # maxPrice: "0.10"             # Default: on-demand price
  # env:
  #   K6_BATCH: "20"
  # arguments: ["--vus", "10"]
  # userDataExtra: |
  #   echo "custom setup"

execution:
  subnets: [subnet-xxx]
  securityGroups: [sg-xxx]
  assignPublicIP: true
  region: ap-northeast-1
  timeout: 30m
  ssmEnabled: true
  # eipAllocationIDs:              # For WAF IP-based allowlisting
  #   - eipalloc-xxxxxxxxxxxxxxxxx

output:
  statsd:
    address: "datadog-agent.service.local:8125"
    enabledTags: true
    namespace: "k6."

cleanup: always  # always | on-success | never
```

### Configuration Reference

| Field | Default | Description |
| :--- | :--- | :--- |
| `name` | (required) | Test run identifier. Used for EC2 instance Name tags, S3 script path, and CloudWatch log group name. |
| `labels` | `{}` | Key-value labels for metadata (reserved for future use). |

**script** — k6 test script source (exactly one required).

| Field | Default | Description |
| :--- | :--- | :--- |
| `script.localFile` | — | Local file path. Uploaded to S3 before launch. |
| `script.s3` | — | Existing S3 URI (`s3://bucket/key`). No upload needed. |
| `script.inline` | — | Inline script content. Uploaded to S3 before launch. |

**runner** — EC2 instance and k6 execution settings.

| Field | Default | Description |
| :--- | :--- | :--- |
| `runner.instanceType` | `c5.xlarge` | EC2 instance type to launch. |
| `runner.parallelism` | `1` | Number of EC2 instances to launch (1–100). |
| `runner.ami` | latest AL2023 | AMI ID. If omitted, resolves the latest Amazon Linux 2023 AMI. |
| `runner.k6Version` | `latest` | k6 version to install from GitHub Releases. |
| `runner.rootVolumeSize` | `20` | EBS root volume size in GiB (gp3, encrypted). |
| `runner.iamInstanceProfile` | — | IAM instance profile name. Required for S3 script access. |
| `runner.env` | `{}` | Environment variables exported to the k6 process. |
| `runner.arguments` | `[]` | Additional CLI arguments passed to `k6 run`. |
| `runner.userDataExtra` | — | Custom shell script injected into UserData (runs after k6 install, before execution). |
| `runner.spot.enabled` | `false` | Request Spot Instances. |
| `runner.spot.maxPrice` | on-demand price | Maximum Spot bid price. |
| `runner.spot.fallbackToOnDemand` | `false` | Retry with On-Demand if Spot capacity is unavailable. |

**execution** — Network and runtime settings.

| Field | Default | Description |
| :--- | :--- | :--- |
| `execution.subnets` | (required) | Subnet IDs. Instances are distributed round-robin. |
| `execution.securityGroups` | `[]` | Security group IDs. Must allow SSM agent traffic (port 443). |
| `execution.assignPublicIP` | `false` | Assign a public IP to each instance. |
| `execution.region` | — | AWS region for all API calls. |
| `execution.timeout` | `30m` | Timeout for the entire test run (prepare through cleanup). |
| `execution.ssmEnabled` | `true` | `true`: execute k6 via SSM Run Command (real-time control). `false`: execute k6 inline in UserData (tag-based completion polling). |
| `execution.eipAllocationIDs` | `[]` | Pre-allocated Elastic IP allocation IDs to associate. Count must be >= `parallelism`. |

**output** — Metrics backend.

| Field | Default | Description |
| :--- | :--- | :--- |
| `output.statsd.address` | — | StatsD endpoint (`host:port`). Adds `--out statsd` to k6. |
| `output.statsd.enabledTags` | `false` | Enable tag support in StatsD output. |
| `output.statsd.namespace` | — | Metric name prefix (e.g., `k6.`). |

**cleanup** — Instance termination policy.

| Value | Description |
| :--- | :--- |
| `always` (default) | Always terminate instances after the test. |
| `on-success` | Terminate only if k6 exits successfully. Keep instances on failure for debugging. |
| `never` | Never terminate. Requires manual `k6-ec2 cleanup --force`. |

## CLI

| Command | Description |
| :--- | :--- |
| `k6-ec2 run -f config.yaml` | Run full lifecycle (prepare → launch → execute → cleanup) |
| `k6-ec2 prepare -f config.yaml` | Upload script to S3 and resolve AMI |
| `k6-ec2 launch -f config.yaml` | Launch EC2 instances |
| `k6-ec2 execute -f config.yaml` | Execute k6 on running instances |
| `k6-ec2 cleanup -f config.yaml` | Terminate instances |
| `k6-ec2 validate -f config.yaml` | Validate config file |
| `k6-ec2 logs --test-name X -f` | Stream CloudWatch Logs |
| `k6-ec2 init -o config.yaml` | Generate a sample config file |
| `k6-ec2 version` | Show version |

All configuration is driven by the YAML file (`-f`). Individual config values cannot be overridden via CLI flags.

Pipeline commands (`prepare` → `launch` → `execute` → `cleanup`) share state through a state file (`--state`, default: `.k6-ec2-state.json`).

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
