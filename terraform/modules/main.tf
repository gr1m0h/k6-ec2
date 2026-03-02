data "aws_region" "current" {}
data "aws_caller_identity" "current" {}

locals {
  tags        = merge(var.tags, { "managed-by" = "k6-ec2" })
  bucket_name = var.script_bucket_name != "" ? var.script_bucket_name : "${var.name}-scripts-${data.aws_caller_identity.current.account_id}-${data.aws_region.current.name}"
}

# S3 Bucket for Scripts
resource "aws_s3_bucket" "scripts" {
  bucket = local.bucket_name
  tags   = local.tags
}

resource "aws_s3_bucket_lifecycle_configuration" "scripts" {
  bucket = aws_s3_bucket.scripts.id
  rule {
    id     = "cleanup"
    status = "Enabled"
    filter {
      prefix = "k6-ec2/"
    }
    expiration {
      days = var.s3_expiration_days
    }
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "scripts" {
  bucket = aws_s3_bucket.scripts.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "scripts" {
  bucket                  = aws_s3_bucket.scripts.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# IAM: EC2 Instance Role
resource "aws_iam_role" "runner" {
  name = "${var.name}-runner"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole",
        Effect = "Allow",
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      }
    ]
  })
  tags = local.tags
}

resource "aws_iam_instance_profile" "runner" {
  name = "${var.name}-runner"
  role = aws_iam_role.runner.name
  tags = local.tags
}

# S3: Download scripts
resource "aws_iam_role_policy" "runner_s3" {
  name = "${var.name}-runner-s3"
  role = aws_iam_role.runner.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow",
        Action   = ["s3:GetObject"],
        Resource = "${aws_s3_bucket.scripts.arn}/*"
      }
    ]
  })
}

# SSM: Managed instance core (for SSM Run Command)
resource "aws_iam_role_policy_attachment" "runner_ssm" {
  role       = aws_iam_role.runner.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

# CloudWatch Logs
resource "aws_iam_role_policy" "runner_logs" {
  name = "${var.name}-runner-logs"
  role = aws_iam_role.runner.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow",
        Action   = ["logs:CreateLogStream", "logs:PutLogEvents"],
        Resource = "${aws_cloudwatch_log_group.k6.arn}:*"
      }
    ]
  })
}

# EC2: Self-tagging (for user-data mode exit code reporting)
resource "aws_iam_role_policy" "runner_tags" {
  name = "${var.name}-runner-tags"
  role = aws_iam_role.runner.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow",
        Action   = ["ec2:CreateTags"],
        Resource = "arn:aws:ec2:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:instance/*"
        Condition = {
          StringEquals = {
            "ec2:ResourceTag/k6-ec2/managed-by" = "k6-ec2"
          }
        }
      }
    ]
  })
}

# IAM: CLI User Policy
resource "aws_iam_policy" "cli" {
  name = "${var.name}-cli"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "EC2Instances"
        Effect   = "Allow",
        Action   = ["ec2:RunInstances", "ec2:TerminateInstances", "ec2:DescribeInstances", "ec2:DescribeImages", "ec2:CreateTags"]
        Resource = "*"
        Condition = {
          StringEquals = {
            "ec2:ResourceTag/k6-ec2/managed-by" = "k6-ec2"
          }
        }
      },
      {
        Sid      = "EC2Describe"
        Effect   = "Allow",
        Action   = ["ec2:DescribeInstances", "ec2:DescribeImages", "ec2:DescribeSubnets", "ec2:DescribeSecurityGroups"]
        Resource = "*"
      },
      {
        Sid    = "EC2Network"
        Effect = "Allow",
        Action = ["ec2:RunInstances"]
        Resource = [
          "arn:aws:ec2:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:subnet/*",
          "arn:aws:ec2:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:security-group/*",
          "arn:aws:ec2:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:network-interface/*",
          "arn:aws:ec2:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}::image/*",
          "arn:aws:ec2:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:volume/*",
        ]
      },
      {
        Sid      = "SSM"
        Effect   = "Allow",
        Action   = ["ssm:SendCommand", "ssm:GetCommandInvocation", "ssm:CancelCommand", "ssm:DescribeInstanceInformation"]
        Resource = "*"
      },
      {
        Sid      = "S3Scripts"
        Effect   = "Allow",
        Action   = ["s3:PutObject", "s3:GetObject"]
        Resource = "${aws_s3_bucket.scripts.arn}/*"
      },
      {
        Sid      = "IAMPassRole"
        Effect   = "Allow",
        Action   = ["iam:PassRole"]
        Resource = aws_iam_role.runner.arn
      },
      {
        Sid      = "CloudWatchLogs"
        Effect   = "Allow",
        Action   = ["logs:FilterLogEvents", "logs:GetLogEvents", "logs:DescribeLogStreams"]
        Resource = "${aws_cloudwatch_log_group.k6.arn}:*"
      },
    ]
  })
  tags = local.tags
}

# Security Group
resource "aws_security_group" "k6" {
  name_prefix = "${var.name}-"
  vpc_id      = var.vpc_id
  description = "k6-ec2 runner instances"
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
  tags = merge(local.tags, { Name = var.name })
  lifecycle {
    create_before_destroy = true
  }
}

# CloudWatch
resource "aws_cloudwatch_log_group" "k6" {
  name              = var.log_group_prefix
  retention_in_days = var.log_retention_days
  tags              = local.tags
}

# Elastic IPs (for WAF IP-based llowlisting)
resource "aws_eip" "k6" {
  count  = var.eip_count
  domain = "vpc"
  tags   = merge(local.tags, { Name = "${var.name}-runner-${count.index}" })
}

# EIP management policy for CLI user (only when EIPs are enabled)
resource "aws_iam_policy" "cli_eip" {
  count = var.eip_count > 0 ? 1 : 0
  name  = "${var.name}-cli-eip"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "EIPManagement"
        Effect   = "Allow",
        Action   = ["ec2:AssociateAddress", "ec2:DisassociateAddress", "ec2:DescribeAddresses"],
        Resource = "*"
      }
    ]
  })
  tags = local.tags
}
