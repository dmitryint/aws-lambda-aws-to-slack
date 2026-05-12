locals {
  release_zip = "${path.module}/../../package/lambda-aws-to-slack_linux_arm64.zip"

  enable_kms_decrypt = var.kms_decrypt_key_arn != null
  enable_chart_s3    = var.chart_bucket_name != null
  enable_dedup       = var.dedup_table_name != null
}

data "aws_partition" "current" {}

data "aws_caller_identity" "current" {}

data "aws_region" "current" {}

data "aws_iam_policy_document" "assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "this" {
  name               = "${var.function_name}-role"
  assume_role_policy = data.aws_iam_policy_document.assume.json
  tags               = var.tags
}

resource "aws_iam_role_policy_attachment" "basic" {
  role       = aws_iam_role.this.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy_attachment" "codecommit_read" {
  role       = aws_iam_role.this.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AWSCodeCommitReadOnly"
}

data "aws_iam_policy_document" "kms_decrypt" {
  count = local.enable_kms_decrypt ? 1 : 0

  statement {
    sid       = "DecryptEnvVars"
    actions   = ["kms:Decrypt"]
    resources = [var.kms_decrypt_key_arn]
  }
}

resource "aws_iam_role_policy" "kms_decrypt" {
  count = local.enable_kms_decrypt ? 1 : 0

  name   = "${var.function_name}-kms-decrypt"
  role   = aws_iam_role.this.id
  policy = data.aws_iam_policy_document.kms_decrypt[0].json
}

data "aws_iam_policy_document" "s3_chart_bucket" {
  count = local.enable_chart_s3 ? 1 : 0

  statement {
    sid = "ChartBucketObjectIO"
    actions = [
      "s3:PutObject",
      "s3:GetObject",
    ]
    resources = ["arn:${data.aws_partition.current.partition}:s3:::${var.chart_bucket_name}/*"]
  }
}

resource "aws_iam_role_policy" "s3_chart_bucket" {
  count = local.enable_chart_s3 ? 1 : 0

  name   = "${var.function_name}-s3-chart-bucket"
  role   = aws_iam_role.this.id
  policy = data.aws_iam_policy_document.s3_chart_bucket[0].json
}

data "aws_iam_policy_document" "dynamodb_dedup" {
  count = local.enable_dedup ? 1 : 0

  statement {
    sid       = "DedupTablePut"
    actions   = ["dynamodb:PutItem"]
    resources = ["arn:${data.aws_partition.current.partition}:dynamodb:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:table/${var.dedup_table_name}"]
  }
}

resource "aws_iam_role_policy" "dynamodb_dedup" {
  count = local.enable_dedup ? 1 : 0

  name   = "${var.function_name}-dynamodb-dedup"
  role   = aws_iam_role.this.id
  policy = data.aws_iam_policy_document.dynamodb_dedup[0].json
}

data "aws_iam_policy_document" "cloudwatch_metric_widget" {
  statement {
    sid = "CloudWatchWidgetAndAlarms"
    actions = [
      "cloudwatch:GetMetricWidgetImage",
      "cloudwatch:DescribeAlarms",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "cloudwatch_metric_widget" {
  name   = "${var.function_name}-cloudwatch-metric-widget"
  role   = aws_iam_role.this.id
  policy = data.aws_iam_policy_document.cloudwatch_metric_widget.json
}

resource "aws_cloudwatch_log_group" "this" {
  name              = "/aws/lambda/${var.function_name}"
  retention_in_days = var.log_retention_days
  tags              = var.tags
}

resource "aws_lambda_function" "this" {
  function_name = var.function_name
  role          = aws_iam_role.this.arn

  filename      = local.release_zip
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]

  memory_size = var.memory_size
  timeout     = var.timeout_seconds

  # kms_key_arn stays set even when env-var KMS auto-decrypt is opt-in:
  # env vars are still encrypted at rest with this CMK regardless of whether
  # their plaintext values are also ciphertext.
  kms_key_arn = var.kms_decrypt_key_arn

  # source_code_hash is intentionally omitted in this example so `tofu validate`
  # succeeds without the zip needing to exist on disk. A real deployment sets:
  #   source_code_hash = filebase64sha256(local.release_zip)

  environment {
    variables = {
      SLACK_HOOK_URL      = var.slack_hook_url
      SLACK_CHANNEL       = var.slack_channel == null ? "" : var.slack_channel
      CHART_BUCKET_NAME   = var.chart_bucket_name == null ? "" : var.chart_bucket_name
      CHART_BUCKET_REGION = var.chart_bucket_region == null ? "" : var.chart_bucket_region
      DEDUP_TABLE_NAME    = var.dedup_table_name == null ? "" : var.dedup_table_name
      DEDUP_TTL_DAYS      = tostring(var.dedup_ttl_days)
      HIDE_AWS_LINKS      = tostring(var.hide_aws_links)
    }
  }

  tags = var.tags

  depends_on = [
    aws_cloudwatch_log_group.this,
    aws_iam_role_policy_attachment.basic,
  ]
}
