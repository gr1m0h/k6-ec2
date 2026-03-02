# k6-ec2

![](./images/logo.svg)

Distributed k6 load testing on AWS EC2 - run k6 directly on EC2 instances with Spot support, no ECS required.

> Have an ECS cluster? See **[k6-ec2](https://github.com/gr1m0h/k6-ecs)** for ECS Fargate/EC2 launch type support.

## Overview

k6-ec2 orchestrates distributed [k6](https://k6.io/) load tests across multiple EC2 instances. It launches instances (on-demand or Spot), uploads your test script, executes k6 via SSM Run Command, streams logs, and cleans up automatically.

## Architecture

![](./images/architecture.svg)

## Features

- **Spot instances** - automatic fallback to on-demand if capacity unavailable
- **SSM Run Command** - no SSH keys needed, CloudWatch Logs integration
- **Auto AMI resolution** - latest Amazon Linux 2023
- **Multi-subnet distribution** - round-robin placement across AZs
- **IMDSv2 enforced**, EBS encryption, least-privilege IAM
- **Declarative YAML** - same configuration model as k6-ecs

## Quick Start

```bash
# 1. Install
go install github.com/k6-distributed/k6-ec2/cmd/k6-ec2@latest

# 2. Set up infrastructure
cd terraform
terraform init && terraform apply -var="vpc_id=vpc-xxx" -var='subnet_ids=["subnet-aaa"]'

# 3. Run
k6-ec2 init -o testrun.yaml # generate sample config
k6-ec2 run -f testrun.yaml # execute
```

## Configuration

```yaml
apiVersion: k6-ec2.io/v1alpha1
kind: EC2TestRun
metadata:
  name: my-load-test
spec:
  script:
    localFile: ./scripts/test.js # or s3: / inline:
  runner:
    instanceType: c5.xlarge # any EC2 instance type
    parallelism: 8 # number of instances
    k6Version: latest # k6 version to install
    iamInstanceProfile: k6-ec2-runner
    spot:
      enabled: true
      fallbackToOnDemand: true
    env:
      K6_BATCH: "50"
  execution:
    subnets: [subnet-xxx, subnet-yyy]
    securityGroups: [sg-xxx]
    assignPublicIP: true
    region: ap-northeast-1
    timeout: 45m
    ssmEnabled: true # recommended
  output:
    statsd:
      address: "dd-agent:8125"
      enableTags: true
  cleanup:
    policy: always # always | on-success | never
```

## CLI

| Command                          | Description                    |
| :------------------------------- | :----------------------------- |
| `k6-ec2 run -f config.yaml`      | Execute a distributed test run |
| `k6-ec2 validate -f config.yaml` | Validate configuration         |
| `k6-ec2 logs --test-name X -f`   | Stream/view logs               |
| `k6-ec2 init -o config.yaml`     | Generate sample config         |
| `k6-ec2 version`                 | Print version                  |

## Spot vs on-demand

| Feature      | Spot                   | On-Demand          |
| :----------- | :--------------------- | :----------------- |
| Cost         | Up to 90% savings      | Full price         |
| Availability | May be interrupted     | Always available   |
| Use case     | Cost-effective testing | Critical test runs |

## SSM vs UserData Execution

| Mode              | Description                                  | Recommended          |
| :---------------- | :------------------------------------------- | :------------------- |
| **SSM** (default) | Execute via SSM Run Command, CloudWatch Logs | Yes                  |
| **UserData**      | Execute in instance boot script              | For custom workflows |
