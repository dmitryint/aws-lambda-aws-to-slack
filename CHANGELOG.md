# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- CloudWatch alarm charts for alarms whose statistic is not a single word. Alarm SNS payloads publish named statistics upper-cased (`SAMPLECOUNT`), which was rendered as `Samplecount` and rejected by `GetMetricWidgetImage` with a `ValidationError`, dropping the chart from the alert. Statistics are now normalized case-insensitively to the spelling the metric widget schema accepts, percentiles are lower-cased (`P99` → `p99`), and an empty statistic omits `stat` instead of sending an empty string.
- Slack alerts that carry one oversized text value. A notification whose title, summary, field, footnote or chart title exceeded a Block Kit text limit — for example a GuardDuty resource JSON dump in a field or a long AWS Health description — was rejected by the webhook with `400 invalid_attachments`, and the whole alert was lost. Section text and context elements are now cut to 3000 characters, section fields and image alt text to 2000, keeping the leading content and ending with `…`; a cut never splits a `<url|label>` link.
- Inspector2 findings silenced after a failed Slack delivery. The dedup key was reserved while the finding was parsed, so when the post failed (for example on HTTP 429 after the client's retries) the Lambda retry found the key already taken and dropped the finding as a duplicate for the whole dedup TTL. A failed delivery now deletes the reservation so the retry is delivered; a successful delivery still silences repeats. The dedup table's IAM policy must now also allow `dynamodb:DeleteItem` — without it the release is logged as failed and the retry stays deduped as before.

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
