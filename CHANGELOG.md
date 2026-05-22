# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-05-22

### Added

- Initial public release.
- AWS Lambda handler that forwards AWS service notifications to Slack incoming webhooks.
- Source parsers: Auto Scaling, AWS Health, AWS Batch, Elastic Beanstalk, CloudFormation, CloudWatch alarms, CodeBuild, CodeCommit (pull-request and repository), CodeDeploy (SNS and EventBridge), CodePipeline (state changes and manual approval), ECS, GuardDuty, Inspector (classic), Inspector2, RDS, SES (bounce / complaint / received), plus a generic fallback formatter.
- KMS-encrypted configuration for `SLACK_HOOK_URL` and `SLACK_CHANNEL`, auto-detected by base64 ciphertext magic bytes and decrypted at cold start.
- CloudWatch alarm rendering with `AlarmDescription` section blocks and inline metric chart images written to an S3 bucket with configurable TTL and SSE algorithm.
- DynamoDB-backed dedup for Inspector2 findings (TTL configurable).
- Transport-neutral notification model with Slack as the default renderer.
- Optional suppression of AWS console links via `HIDE_AWS_LINKS`.
- Build pipeline producing `linux/amd64` and `linux/arm64` `bootstrap` zips for the `provided.al2023` runtime.
- Terraform example under `examples/lambda/` showing function, role, and inline IAM.
- GitHub Actions: tests on PR, release artifacts on GitHub Release.

[Unreleased]: https://github.com/dmitryint/aws-lambda-aws-to-slack/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/dmitryint/aws-lambda-aws-to-slack/releases/tag/v0.1.0
