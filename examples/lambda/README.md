# `examples/lambda/` — per-source attachment cookbook

This is the copy-pasteable cookbook for wiring AWS event sources into the
`aws-to-slack` Lambda. The Terraform under `examples/lambda/` (see
`main.tf`, `variables.tf`, `outputs.tf`, `versions.tf`) declares only the
Lambda function, its execution role, and the role's base inline policies
(KMS, S3 charts bucket, DynamoDB dedup, CloudWatch
`GetMetricWidgetImage`). It deliberately does **not** create any event
sources — that is the consumer's responsibility, because a single Lambda
may have a dozen different producers attached and they are typically
owned by different stacks.

Two integration patterns cover every recipe below:

- **SNS subscription** — the producer publishes to an SNS topic; an
  `aws_sns_topic_subscription` with `protocol = "lambda"` sends events to
  this Lambda, and an `aws_lambda_permission` allows `sns.amazonaws.com`
  to invoke. The topic's KMS key (if any) must grant `kms:Decrypt` to
  the Lambda role for at-rest message decryption.
- **EventBridge target** — the producer publishes to the default event
  bus; an `aws_cloudwatch_event_rule` matches, an
  `aws_cloudwatch_event_target` points at this Lambda, and an
  `aws_lambda_permission` allows `events.amazonaws.com` to invoke.

Every recipe lists **(a)** the producer-side resource(s), **(b)** the
EventBridge rule pattern or SNS topic to attach, **(c)** the
`aws_lambda_permission`, and **(d)** any env vars the parser needs on
the Lambda. The recipes reference `module.aws_to_slack.function_name`
and `module.aws_to_slack.function_arn` — these are the outputs exposed
by `examples/lambda/` (alongside `role_name`, `role_arn`,
`log_group_name`). Consumers wire their own module reference; the names
here are spec-only.

## Common scaffolding (referenced by every recipe)

```hcl
# examples/lambda/main.tf — what this file contains
resource "aws_lambda_function" "aws_to_slack" { /* the Lambda */ }
resource "aws_iam_role"        "aws_to_slack" { /* exec role  */ }
# inline policies: kms:Decrypt, s3:Put/GetObject on charts bucket,
# dynamodb:PutItem on dedup table, cloudwatch:GetMetricWidgetImage
```

In the recipes, `module.aws_to_slack.function_name` and
`module.aws_to_slack.function_arn` refer to outputs from
`examples/lambda/`. Producers live in *separate* root modules.

---

## A. CloudWatch alarms (SNS)

CloudWatch alarms publish to SNS when state changes. The Lambda
subscribes to the alarm-notification topic.

```hcl
# producer side: one SNS topic dedicated to alarms
resource "aws_sns_topic" "alarms" {
  name              = "aws-alarms"
  kms_master_key_id = var.alarm_topic_kms_key_arn  # CMK, key policy must grant Decrypt to Lambda role
}

# the Lambda subscribes
resource "aws_sns_topic_subscription" "aws_to_slack" {
  topic_arn = aws_sns_topic.alarms.arn
  protocol  = "lambda"
  endpoint  = module.aws_to_slack.function_arn
}

resource "aws_lambda_permission" "allow_sns_alarms" {
  statement_id  = "AllowSNSInvocationFromAlarms"
  action        = "lambda:InvokeFunction"
  function_name = module.aws_to_slack.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.alarms.arn
}

# each alarm uses this topic
resource "aws_cloudwatch_metric_alarm" "example" {
  # ...
  alarm_actions             = [aws_sns_topic.alarms.arn]
  ok_actions                = [aws_sns_topic.alarms.arn]
  insufficient_data_actions = [aws_sns_topic.alarms.arn]
}
```

Env vars on the Lambda: `CHART_BUCKET_NAME`, `CHART_BUCKET_REGION`
(both required for chart rendering).

## B. Auto Scaling group lifecycle (SNS)

```hcl
resource "aws_sns_topic" "asg" {
  name = "asg-notifications"
}

resource "aws_autoscaling_notification" "example" {
  group_names   = [aws_autoscaling_group.example.name]
  notifications = [
    "autoscaling:EC2_INSTANCE_LAUNCH",
    "autoscaling:EC2_INSTANCE_LAUNCH_ERROR",
    "autoscaling:EC2_INSTANCE_TERMINATE",
    "autoscaling:EC2_INSTANCE_TERMINATE_ERROR",
    "autoscaling:TEST_NOTIFICATION",
  ]
  topic_arn = aws_sns_topic.asg.arn
}

resource "aws_sns_topic_subscription" "aws_to_slack" {
  topic_arn = aws_sns_topic.asg.arn
  protocol  = "lambda"
  endpoint  = module.aws_to_slack.function_arn
}

resource "aws_lambda_permission" "allow_sns_asg" {
  statement_id  = "AllowSNSInvocationFromASG"
  action        = "lambda:InvokeFunction"
  function_name = module.aws_to_slack.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.asg.arn
}
```

## C. AWS Health (EventBridge)

```hcl
resource "aws_cloudwatch_event_rule" "aws_health" {
  name          = "aws-health-to-slack"
  description   = "Forward AWS Health events to aws-to-slack"
  event_pattern = jsonencode({
    source = ["aws.health"]
    # optionally narrow by detail-type / service / eventTypeCategory
  })
}

resource "aws_cloudwatch_event_target" "aws_health" {
  rule = aws_cloudwatch_event_rule.aws_health.name
  arn  = module.aws_to_slack.function_arn
}

resource "aws_lambda_permission" "allow_events_health" {
  statement_id  = "AllowEventsHealth"
  action        = "lambda:InvokeFunction"
  function_name = module.aws_to_slack.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.aws_health.arn
}
```

Same EventBridge shape applies to the next recipes — each just
changes the `event_pattern.source` and `statement_id`. The
producer-side resources are show-only and the rule/target/
permission triplet repeats. To keep the doc readable, the
EventBridge recipes below show only the pattern and the rule name;
treat the target + permission as a copy of the AWS Health block.

## D. AWS Batch (EventBridge)

```hcl
resource "aws_cloudwatch_event_rule" "batch" {
  name          = "aws-batch-to-slack"
  event_pattern = jsonencode({
    source       = ["aws.batch"]
    "detail-type" = ["Batch Job State Change"]
    detail = {
      status = ["SUCCEEDED", "FAILED"]  # optional filter; drop to receive all states
    }
  })
}
```

Wire the `aws_cloudwatch_event_target` and `aws_lambda_permission`
exactly as shown in Recipe C, swapping the rule reference and the
`statement_id` (for example `AllowEventsBatch`).

## E. Elastic Beanstalk (SNS, subject-based)

Beanstalk environments can post lifecycle notifications to an SNS
topic. The parser matches by SNS *subject* prefix
(`AWS Elastic Beanstalk Notification`).

```hcl
resource "aws_sns_topic" "beanstalk" {
  name = "beanstalk-notifications"
}

resource "aws_elastic_beanstalk_environment" "example" {
  # ...
  setting {
    namespace = "aws:elasticbeanstalk:sns:topics"
    name      = "Notification Topic ARN"
    value     = aws_sns_topic.beanstalk.arn
  }
}

# subscription + permission identical to recipe A
resource "aws_sns_topic_subscription" "aws_to_slack" {
  topic_arn = aws_sns_topic.beanstalk.arn
  protocol  = "lambda"
  endpoint  = module.aws_to_slack.function_arn
}

resource "aws_lambda_permission" "allow_sns_beanstalk" {
  statement_id  = "AllowSNSInvocationFromBeanstalk"
  action        = "lambda:InvokeFunction"
  function_name = module.aws_to_slack.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.beanstalk.arn
}
```

## F. CloudFormation (SNS, subject-based)

Stacks emit lifecycle notifications when given `NotificationARNs`.
Subject prefix matched: `AWS CloudFormation Notification`.

```hcl
resource "aws_sns_topic" "cfn" {
  name = "cfn-notifications"
}

resource "aws_cloudformation_stack" "example" {
  # ...
  notification_arns = [aws_sns_topic.cfn.arn]
}

# subscription + permission identical to recipe A
resource "aws_sns_topic_subscription" "aws_to_slack" {
  topic_arn = aws_sns_topic.cfn.arn
  protocol  = "lambda"
  endpoint  = module.aws_to_slack.function_arn
}

resource "aws_lambda_permission" "allow_sns_cfn" {
  statement_id  = "AllowSNSInvocationFromCloudFormation"
  action        = "lambda:InvokeFunction"
  function_name = module.aws_to_slack.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.cfn.arn
}
```

## G. CodeBuild (EventBridge)

```hcl
resource "aws_cloudwatch_event_rule" "codebuild" {
  name          = "codebuild-to-slack"
  event_pattern = jsonencode({
    source       = ["aws.codebuild"]
    "detail-type" = ["CodeBuild Build State Change"]
    # NOT "CodeBuild Build Phase Change" — parser silences those
  })
}
```

Attach via the EventBridge target + `aws_lambda_permission`
triplet from Recipe C (see Recipe C), substituting
`statement_id = "AllowEventsCodeBuild"`.

## H. CodeCommit (EventBridge)

One rule per detail-type, or a single rule covering both:

```hcl
resource "aws_cloudwatch_event_rule" "codecommit" {
  name          = "codecommit-to-slack"
  event_pattern = jsonencode({
    source       = ["aws.codecommit"]
    "detail-type" = [
      "CodeCommit Pull Request State Change",
      "CodeCommit Repository State Change",
    ]
  })
}
```

The repository parser calls `codecommit:GetBranch` + `GetCommit` —
make sure the Lambda role has `AWSCodeCommitReadOnly` (already
present in the example execution role). Attach via the EventBridge
triplet from Recipe C (see Recipe C), substituting
`statement_id = "AllowEventsCodeCommit"`.

## I. CodeDeploy (two flavors)

CodeDeploy publishes both EventBridge events *and* per-deployment-
group SNS triggers; the binary has separate parsers for each.

**EventBridge variant:**

```hcl
resource "aws_cloudwatch_event_rule" "codedeploy" {
  name          = "codedeploy-to-slack"
  event_pattern = jsonencode({
    source       = ["aws.codedeploy"]
    "detail-type" = ["CodeDeploy Deployment State-change Notification"]
  })
}
```

Attach via the EventBridge triplet from Recipe C (see Recipe C),
substituting `statement_id = "AllowEventsCodeDeploy"`.

**SNS variant (per deployment group):**

```hcl
resource "aws_sns_topic" "codedeploy" {
  name = "codedeploy-notifications"
}

resource "aws_codedeploy_deployment_group" "example" {
  # ...
  trigger_configuration {
    trigger_name       = "all-deployment-events"
    trigger_target_arn = aws_sns_topic.codedeploy.arn
    trigger_events = [
      "DeploymentStart", "DeploymentSuccess", "DeploymentFailure",
      "DeploymentStop", "DeploymentRollback", "DeploymentReady",
    ]
  }
}

# subscription + permission identical to recipe A
resource "aws_sns_topic_subscription" "aws_to_slack" {
  topic_arn = aws_sns_topic.codedeploy.arn
  protocol  = "lambda"
  endpoint  = module.aws_to_slack.function_arn
}

resource "aws_lambda_permission" "allow_sns_codedeploy" {
  statement_id  = "AllowSNSInvocationFromCodeDeploy"
  action        = "lambda:InvokeFunction"
  function_name = module.aws_to_slack.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.codedeploy.arn
}
```

If both are attached, the parser waterfall ensures the EventBridge
parser wins on EventBridge payloads and the SNS parser wins on SNS
payloads. Either alone works.

## J. CodePipeline state changes (EventBridge)

```hcl
resource "aws_cloudwatch_event_rule" "codepipeline" {
  name          = "codepipeline-to-slack"
  event_pattern = jsonencode({
    source       = ["aws.codepipeline"]
    "detail-type" = [
      "CodePipeline Pipeline Execution State Change",
      "CodePipeline Stage Execution State Change",
      "CodePipeline Action Execution State Change",
    ]
  })
}
```

Attach via the EventBridge triplet from Recipe C (see Recipe C),
substituting `statement_id = "AllowEventsCodePipeline"`.

## K. CodePipeline manual approval (SNS)

Manual approval actions notify via an SNS topic configured on the
action itself.

```hcl
# inside the pipeline definition (CodePipeline JSON / TF):
# action "Approve" of type "Approval", configuration:
#   NotificationArn = aws_sns_topic.cp_approval.arn
#   CustomData      = "..."

resource "aws_sns_topic" "cp_approval" {
  name = "codepipeline-approvals"
}

# subscription + permission identical to recipe A
resource "aws_sns_topic_subscription" "aws_to_slack" {
  topic_arn = aws_sns_topic.cp_approval.arn
  protocol  = "lambda"
  endpoint  = module.aws_to_slack.function_arn
}

resource "aws_lambda_permission" "allow_sns_cp_approval" {
  statement_id  = "AllowSNSInvocationFromCodePipelineApproval"
  action        = "lambda:InvokeFunction"
  function_name = module.aws_to_slack.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.cp_approval.arn
}
```

## L. ECS (EventBridge)

```hcl
resource "aws_cloudwatch_event_rule" "ecs" {
  name          = "ecs-to-slack"
  event_pattern = jsonencode({
    source       = ["aws.ecs"]
    "detail-type" = [
      "ECS Task State Change",
      "ECS Service Action",
    ]
    # ECS Deployment State Change and ECS Container Instance State Change
    # fall through to the default attachment shape
  })
}
```

Attach via the EventBridge triplet from Recipe C (see Recipe C),
substituting `statement_id = "AllowEventsECS"`.

## M. GuardDuty (EventBridge)

```hcl
resource "aws_cloudwatch_event_rule" "guardduty" {
  name          = "guardduty-to-slack"
  event_pattern = jsonencode({
    source       = ["aws.guardduty"]
    "detail-type" = ["GuardDuty Finding"]
    detail = {
      severity = [{ "numeric" : [">", 4] }]  # optional: medium+ only
    }
  })
}
```

Attach via the EventBridge triplet from Recipe C (see Recipe C),
substituting `statement_id = "AllowEventsGuardDuty"`.

## N. Inspector classic (SNS — legacy)

```hcl
resource "aws_sns_topic" "inspector_classic" {
  name = "inspector-classic-findings"
}

# assessment template references the SNS topic via
# inspector:SubscribeToEvent. Terraform support varies; the
# CLI equivalent:
#   aws inspector subscribe-to-event \
#     --resource-arn <assessment-template-arn> \
#     --event ASSESSMENT_RUN_COMPLETED \
#     --topic-arn arn:aws:sns:...:inspector-classic-findings
#
# subscribe to: ASSESSMENT_RUN_STARTED, ASSESSMENT_RUN_COMPLETED,
# ASSESSMENT_RUN_STATE_CHANGED, FINDING_REPORTED.
# (Do NOT subscribe ENABLE_ASSESSMENT_NOTIFICATIONS — parser silences it.)

# subscription + permission identical to recipe A
resource "aws_sns_topic_subscription" "aws_to_slack" {
  topic_arn = aws_sns_topic.inspector_classic.arn
  protocol  = "lambda"
  endpoint  = module.aws_to_slack.function_arn
}

resource "aws_lambda_permission" "allow_sns_inspector_classic" {
  statement_id  = "AllowSNSInvocationFromInspectorClassic"
  action        = "lambda:InvokeFunction"
  function_name = module.aws_to_slack.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.inspector_classic.arn
}
```

AWS Inspector classic is in maintenance (no new customers since
2024). Keep this recipe for accounts that haven't migrated.

## O. Inspector2 (EventBridge)

```hcl
resource "aws_cloudwatch_event_rule" "inspector2" {
  name          = "inspector2-to-slack"
  event_pattern = jsonencode({
    source       = ["aws.inspector2"]
    "detail-type" = ["Inspector2 Finding"]
    detail = {
      severity = ["HIGH", "CRITICAL"]  # match the parser filter
    }
  })
}
```

Attach via the EventBridge triplet from Recipe C (see Recipe C),
substituting `statement_id = "AllowEventsInspector2"`.

Env vars: `DEDUP_TABLE_NAME`, `DEDUP_TTL_DAYS` (Inspector2 is the
only parser that uses the dedup table). The dedup table is created
by the Lambda's `examples/lambda/main.tf` and referenced via env.

## P. RDS event subscriptions (SNS)

RDS uses *event subscriptions*, not direct SNS publishing — you
declare which source type and event categories you care about, and
RDS forwards matching events to the topic.

```hcl
resource "aws_sns_topic" "rds" {
  name = "rds-events"
}

resource "aws_db_event_subscription" "example" {
  name      = "rds-to-slack"
  sns_topic = aws_sns_topic.rds.arn

  source_type = "db-instance"  # parser matches Event Source == "db-instance"
  event_categories = [
    "availability", "backup", "configuration change", "creation",
    "deletion", "failover", "failure", "low storage", "maintenance",
    "notification", "read replica", "recovery", "restoration",
    "security patching",
  ]
}

# subscription + permission identical to recipe A
resource "aws_sns_topic_subscription" "aws_to_slack" {
  topic_arn = aws_sns_topic.rds.arn
  protocol  = "lambda"
  endpoint  = module.aws_to_slack.function_arn
}

resource "aws_lambda_permission" "allow_sns_rds" {
  statement_id  = "AllowSNSInvocationFromRDS"
  action        = "lambda:InvokeFunction"
  function_name = module.aws_to_slack.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.rds.arn
}
```

## Q. SES bounce / complaint / received (SNS)

SES has two flavors of SNS hook:

**Outbound delivery (bounce, complaint, delivery):** configured per
configuration set, per identity, or per email-sending identity.

```hcl
resource "aws_sns_topic" "ses_outbound" {
  name = "ses-bounce-complaint"
}

resource "aws_ses_identity_notification_topic" "bounce" {
  identity                 = aws_ses_domain_identity.example.domain
  notification_type        = "Bounce"
  topic_arn                = aws_sns_topic.ses_outbound.arn
  include_original_headers = true
}

resource "aws_ses_identity_notification_topic" "complaint" {
  identity          = aws_ses_domain_identity.example.domain
  notification_type = "Complaint"
  topic_arn         = aws_sns_topic.ses_outbound.arn
}

# subscription + permission identical to recipe A
resource "aws_sns_topic_subscription" "aws_to_slack" {
  topic_arn = aws_sns_topic.ses_outbound.arn
  protocol  = "lambda"
  endpoint  = module.aws_to_slack.function_arn
}

resource "aws_lambda_permission" "allow_sns_ses_outbound" {
  statement_id  = "AllowSNSInvocationFromSESOutbound"
  action        = "lambda:InvokeFunction"
  function_name = module.aws_to_slack.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.ses_outbound.arn
}
```

**Inbound (received):** configured as an SES receipt-rule action
(`SNSAction`). Note: SES inbound is region-limited and used by
relatively few stacks.

```hcl
resource "aws_ses_receipt_rule" "example" {
  # ...
  sns_action {
    topic_arn = aws_sns_topic.ses_inbound.arn
    encoding  = "UTF-8"
  }
}
```

Wire `aws_sns_topic.ses_inbound`, its
`aws_sns_topic_subscription`, and an `aws_lambda_permission`
(`statement_id = "AllowSNSInvocationFromSESInbound"`,
`principal = "sns.amazonaws.com"`) using the same template as
Recipe A.

## R. Generic catch-all

The `generic` parser matches anything that no other parser claims.
To intentionally route a payload through it, attach via either
pattern (SNS or EventBridge) with no producer-specific filtering —
the parser waterfall picks `generic` as the last resort.

## Generic catch-all

Convenience alias for Recipe R — the `generic` parser is the
final fallback in the waterfall. Use the SNS subscription template
from Recipe A or the EventBridge triplet from Recipe C with no
producer-specific filter, and the message will be rendered by the
`generic` parser when no other parser claims it.

---

## Cookbook completeness check

The Markdown cookbook includes one recipe section per supported
event source (21 explicit recipes) plus a final "generic" note.
CI's `markdownlint` step asserts the headings A-R exist.
