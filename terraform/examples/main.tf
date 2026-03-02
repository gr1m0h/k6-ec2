################################################################################
# k6-ec2 Complete Example
#
# Load testing topology: EC2 (k6 runners) -> CloudFront -> ALB -> ECS
#
# - EC2 runners are placed in public subnets (avoids NAT Gateway overhead)
# - Session Manager access (no SSH / no inbound SG rules)
# - WAF IP set allowlists runner EIPs on the CloudFront distribution
################################################################################
terraform {
  required_version = ">= 1.5"

  # Uncomment and configure for your organization's remote backend:
  # backend "s3" {
  #   bucket         = "my-org-terraform-state"
  #   key            = "k6-ec2/terraform.tfstate"
  #   region         = "ap-northeast-1"
  #   dynamodb_table = "terraform-locks"
  #   encrypt        = true
  # }
}

provider "aws" {
  region = var.region
}

# WAF v2 IP sets for CloudFront must be created in us-east-1.
provider "aws" {
  alias  = "us_east_1"
  region = "us-east-1"
}

################################################################################
# k6-ec2 Module
################################################################################
module "k6_ec2" {
  source = "../../modules/k6-ec2"

  name       = var.name
  vpc_id     = var.vpc_id
  subnet_ids = var.subnet_ids

  enable_spot            = var.enable_spot
  allowed_instance_types = var.allowed_instance_types
  log_retention_days     = var.log_retention_days
  eip_count              = var.eip_count
  tags                   = var.tags
}

################################################################################
# WAF v2 IP Set (CloudFront scope - us-east-1)
#
# Contains the Elastic IPs of k6 runner instances.
# Reference this IP set in your existing CloudFront WAF WebACL to allow
# load test traffic.
################################################################################

resource "aws_wafv2_ip_set" "k6_runners" {
  count = var.eip_count > 0 ? 1 : 0

  provider           = aws.us_east_1
  name               = "${var.name}-runners"
  scope              = "CLOUDFRONT"
  ip_address_version = "IPV4"
  addresses          = module.k6_ec2.waf_ip_set_cidrs
  tags               = var.tags
}

################################################################################
# WAF WebACL Rule Integration
#
# Add the IP set as an allow rule in your existing CloudFront WAF WebACL.
# Example using aws_wafv2_web_acl:
#
#   rule {
#     name     = "allow-k6-runners"
#     priority = 5    # Adjust priority for your rule set
#
#     action { allow {} }
#
#     statement {
#       ip_set_reference_statement {
#         arn = aws_wafv2_ip_set.k6_runners[0].arn
#       }
#     }
#
#     visibility_config {
#       sampled_requests_enabled   = true
#       cloudwatch_metrics_enabled = true
#       metric_name                = "k6-runners-allow"
#     }
#   }
################################################################################

################################################################################
# IAM Policy Attachments
################################################################################

# Attach CLI policy to your CI/CD role so it can run k6-ec2.
# Replace the role name with your actual CI runner role.
#
# resource "aws_iam_role_policy_attachment" "ci_k6" {
#   role       = "my-ci-runner-role"
#   policy_arn = module.k6_ec2.cli_policy_arn
# }

# When using EIPs, also attach the EIP management policy.
#
# resource "aws_iam_role_policy_attachment" "ci_k6_eip" {
#   count      = var.eip_count > 0 ? 1 : 0
#   role       = "my-ci-runner-role"
#   policy_arn = module.k6_ec2.cli_eip_policy_arn
# }
