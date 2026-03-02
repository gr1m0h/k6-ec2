output "script_bucket" {
  description = "S3 bucket name for k6 scripts. Maps to k6_EC2_SCRIPT_BUCKET env var."
  value       = aws_s3_bucket.scripts.id
}

output "instance_profile_name" {
  description = "IAM instance profile name for k6 runners. Maps to spec.runner.iamInstanceProfile."
  value       = aws_iam_instance_profile.runner.name
}

output "instance_profile_arn" {
  description = "IAM instance profile ARN for k6 runners."
  value       = aws_iam_instance_profile.runner.arn
}

output "runner_role_arn" {
  description = "IAM role ARN assumed by k6 runner EC2 instances."
  value       = aws_iam_role.runner.arn
}

output "cli_policy_arn" {
  description = "IAM policy ARN to attach to the CI/CD role or user that runs  the k6ec2 CLI."
  value       = aws_iam_role.cli.arn
}

output "security_group_id" {
  description = "Security group ID for k6 runners. Map to spec.execution.securityGroups[]."
  value       = aws_security_group.k6.id
}

output "subnet_ids" {
  description = "Subnet IDs passed through for convenience. Maps to spec.execution.subnets[]."
  value       = var.subnet_ids
}

output "log_group_name" {
  description = "CloudWatch log group name. CLI hardcodes /k6-ec2 for log streaming."
  value       = aws_cloudwatch_log_group.k6.name
}

output "cli_eip_policy_arn" {
  description = "IAM policy ARN for EIP management. Attach to the CLI user/role when using EIPs. Null when eip_count is 0."
  value       = var.eip_count > 0 ? aws_iam_policy.cli_eip[0].arn : null
}

output "eip_allocation_ids" {
  description = "EIP allocation IDs for k6 runner instances. Associate with instances at launch time."
  value       = aws_eip.k6[*].allocation_id
}

output "eip_public_ips" {
  description = "EIP public IP addresses for k6 runner instances."
  value       = aws_eip.k6[*].public_id
}

output "waf_ip_set_cidrs" {
  description = "EIP addresses formatted as /32 CIDRs, ready for use in a WAF v2 IP set."
  value       = [for eip in aws_eip.k6 : "${eip.public_ip}/32"]
}

output "sample_config" {
  description = "Sample k6-ec2 YAML config populated with Terraform output values."
  value       = <<-EOT
    apiVersion: k6-ec2.io/v1alpha1
    kind: EC2TestRun
    metadata:
      name: my-load-test
    spec:
      script:
        localFile: ./scripts/test.js
      runner:
        instanceType: c5.xlarge
        parallelism: 4
        iamInstanceProfile: ${aws_iam_instance_profile.runner.name}
        spot:
          enabled: ${var.enable_spot}
          fallbackToOnDemand: true
      execution:
        subnets: ${jsonencode(var.subnet_ids)}
        securityGroups: ["${aws_security_group.k6.id}"]
        assignPublicIP: true
        region: ${data.aws_region.current.name}
        ssmEnabled: true
%{if var.eip_count > 0~}
        eipAllocationIDs: ${jsonencode(aws_eip.k6[*].allocation_id)}
%{else~}
        # eipAllocationIDs: [] # Set eip_count > 0 to enable
%{endif~}
        cleanup:
          policy: always
  EOT
}
