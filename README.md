# aws-lambda-aws-to-slack

AWS Lambda that forwards AWS service notifications to Slack as formatted
incoming-webhook messages. Subscribes to SNS topics, EventBridge rules, or
direct invocations and renders one Slack message per record.

## Supported sources

Auto Scaling, AWS Health, AWS Batch, Elastic Beanstalk, CloudFormation,
CloudWatch alarms, CodeBuild, CodeCommit (pull-request + repository),
CodeDeploy (SNS + EventBridge), CodePipeline (state changes + manual
approval), ECS, GuardDuty, Inspector (classic), Inspector2, RDS, SES
(bounce / complaint / received). Anything else falls through to a generic
formatter.

## Build

```
make package        # produces package/lambda-aws-to-slack_linux_{amd64,arm64}.zip
```

## Test

```
make tidy fmt vet lint test
```

## Deploy

`examples/lambda/` contains a reference Terraform module that declares the
Lambda function, execution role, and the base inline IAM the binary needs
(KMS decrypt, S3 chart-bucket access, DynamoDB dedup, CloudWatch
`GetMetricWidgetImage`). Wire your event sources — SNS subscriptions,
EventBridge targets — per the recipes in `examples/lambda/README.md`.

## Configuration

| Env var               | Purpose                                                          |
|-----------------------|------------------------------------------------------------------|
| `SLACK_HOOK_URL`      | Incoming-webhook URL. Plaintext or KMS-encrypted ciphertext.     |
| `SLACK_CHANNEL`       | Override channel (optional). Plaintext or KMS ciphertext.        |
| `CHART_BUCKET_NAME`   | S3 bucket for CloudWatch alarm chart images.                     |
| `CHART_BUCKET_REGION` | Region of the chart bucket (may differ from `AWS_REGION`).       |
| `DEDUP_TABLE_NAME`    | DynamoDB table for Inspector2 finding dedup.                     |
| `DEDUP_TTL_DAYS`      | TTL on dedup entries.                                            |
| `HIDE_AWS_LINKS`      | `true` or `1` suppresses AWS console links in rendered messages. |

`SLACK_HOOK_URL` and `SLACK_CHANNEL` may be set as plaintext or as a
base64-encoded KMS ciphertext blob. Ciphertext is detected by its leading
magic bytes (`0x01 0x02`) after base64 decoding and decrypted at cold
start. Decryption failures fail the cold start so the configured CloudWatch
alarm on Lambda Errors triggers.

## Layout

- `cmd/aws-to-slack/` — Lambda entrypoint (`bootstrap` for `provided.al2023`).
- `internal/handler/` — wires envelope → router → slack sinks.
- `internal/envelope/` — Lambda payload shape normalization.
- `internal/router/` — ordered parser waterfall.
- `internal/parser/<source>/` — per-AWS-service parsers.
- `internal/slack/` — webhook client, hybrid Block-Kit envelope.
- `internal/console/`, `internal/dedup/`, `internal/kms/`, `internal/config/`
  — supporting packages.
- `samples/` — committed event fixtures used by parser tests.
- `examples/lambda/` — Terraform example showing how to wire the binary up.
