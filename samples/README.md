# samples

Event fixtures used by parser tests. Payloads are sourced from AWS
documentation and the AWS Events Catalog. Anonymization rules: account IDs
→ `123456789012`, resource IDs → synthetic placeholders
(`i-0123456789abcdef0`, etc.), external IPs → `203.0.113.x`, internal IPs
→ `10.0.0.x`, emails → `recipient@example.com` / `sender@example.com`.

## Provenance per fixture

## samples/autoscaling/instance_launch.json

- **Source AWS service**: EC2 Auto Scaling lifecycle notifications
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/autoscaling/ec2/userguide/ASGettingNotifications.html#auto-scaling-sns-message-contents
- **Captured on**: N/A (from docs)
- **Anonymization**: AccountId, ASG ARN, InstanceId, subnet, sns topic ARN
- **Test it drives**: parser/autoscaling/autoscaling_test.go::TestAutoScaling/instance_launch

## samples/autoscaling/instance_launch_error.json

- **Source AWS service**: EC2 Auto Scaling lifecycle notifications
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/autoscaling/ec2/userguide/ASGettingNotifications.html#auto-scaling-sns-message-contents
- **Captured on**: N/A (from docs)
- **Anonymization**: AccountId, ASG ARN, InstanceId, subnet, sns topic ARN
- **Test it drives**: parser/autoscaling/autoscaling_test.go::TestAutoScaling/instance_launch_error

## samples/autoscaling/instance_terminate.json

- **Source AWS service**: EC2 Auto Scaling lifecycle notifications
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/autoscaling/ec2/userguide/ASGettingNotifications.html#auto-scaling-sns-message-contents
- **Captured on**: N/A (from docs)
- **Anonymization**: AccountId, ASG ARN, InstanceId, subnet, sns topic ARN
- **Test it drives**: parser/autoscaling/autoscaling_test.go::TestAutoScaling/instance_terminate

## samples/autoscaling/instance_terminate_error.json

- **Source AWS service**: EC2 Auto Scaling lifecycle notifications
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/autoscaling/ec2/userguide/ASGettingNotifications.html#auto-scaling-sns-message-contents
- **Captured on**: N/A (from docs)
- **Anonymization**: AccountId, ASG ARN, InstanceId, subnet, sns topic ARN
- **Test it drives**: parser/autoscaling/autoscaling_test.go::TestAutoScaling/instance_terminate_error

## samples/autoscaling/test_notification.json

- **Source AWS service**: EC2 Auto Scaling lifecycle notifications
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/autoscaling/ec2/userguide/ASGettingNotifications.html#auto-scaling-sns-message-contents
- **Captured on**: N/A (from docs)
- **Anonymization**: AccountId, ASG ARN, sns topic ARN
- **Test it drives**: parser/autoscaling/autoscaling_test.go::TestAutoScaling/test_notification

## samples/awshealth/abuse_event.json

- **Source AWS service**: AWS Health Dashboard (AWS Health Abuse Event)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/health/latest/ug/cloudwatch-events-health.html (abuse subtype)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, eventArn, InstanceId
- **Test it drives**: parser/awshealth/awshealth_test.go::TestAwsHealth/abuse_event

## samples/awshealth/account_notification.json

- **Source AWS service**: AWS Health Dashboard (accountNotification)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/health/latest/ug/cloudwatch-events-health.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, eventArn
- **Test it drives**: parser/awshealth/awshealth_test.go::TestAwsHealth/account_notification

## samples/awshealth/issue_open_no_end_time.json

- **Source AWS service**: AWS Health Dashboard
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/health/latest/ug/cloudwatch-events-health.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, eventArn
- **Test it drives**: parser/awshealth/awshealth_test.go::TestAwsHealth/issue_open_no_end_time

## samples/awshealth/issue_resolved_with_end_time.json

- **Source AWS service**: AWS Health Dashboard
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/health/latest/ug/cloudwatch-events-health.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, eventArn
- **Test it drives**: parser/awshealth/awshealth_test.go::TestAwsHealth/issue_resolved_with_end_time

## samples/awshealth/multi_entity.json

- **Source AWS service**: AWS Health Dashboard (multiple affected entities)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/health/latest/ug/cloudwatch-events-health.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, eventArn, InstanceIds
- **Test it drives**: parser/awshealth/awshealth_test.go::TestAwsHealth/multi_entity

## samples/awshealth/multi_language_description.json

- **Source AWS service**: AWS Health Dashboard (multi-language eventDescription)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/health/latest/ug/cloudwatch-events-health.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, eventArn
- **Test it drives**: parser/awshealth/awshealth_test.go::TestAwsHealth/multi_language_description

## samples/awshealth/scheduled_change.json

- **Source AWS service**: AWS Health Dashboard (scheduledChange)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/health/latest/ug/cloudwatch-events-health.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, eventArn, InstanceId
- **Test it drives**: parser/awshealth/awshealth_test.go::TestAwsHealth/scheduled_change

## samples/batch/job_failed.json

- **Source AWS service**: AWS Batch
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/batch/latest/userguide/batch_cwe_events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, jobArn, jobId, jobDefinition, jobRoleArn, containerInstanceArn, taskArn
- **Test it drives**: parser/batch/batch_test.go::TestBatch/job_failed

## samples/batch/job_runnable.json

- **Source AWS service**: AWS Batch
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/batch/latest/userguide/batch_cwe_events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, jobArn, jobId, jobDefinition, jobRoleArn
- **Test it drives**: parser/batch/batch_test.go::TestBatch/job_runnable

## samples/batch/job_running.json

- **Source AWS service**: AWS Batch
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/batch/latest/userguide/batch_cwe_events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, jobArn, jobId, jobDefinition, jobRoleArn, containerInstanceArn, taskArn
- **Test it drives**: parser/batch/batch_test.go::TestBatch/job_running

## samples/batch/job_starting.json

- **Source AWS service**: AWS Batch
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/batch/latest/userguide/batch_cwe_events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, jobArn, jobId, jobDefinition, jobRoleArn, containerInstanceArn, taskArn
- **Test it drives**: parser/batch/batch_test.go::TestBatch/job_starting

## samples/batch/job_submitted.json

- **Source AWS service**: AWS Batch
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/batch/latest/userguide/batch_cwe_events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, jobArn, jobId, jobDefinition, jobRoleArn
- **Test it drives**: parser/batch/batch_test.go::TestBatch/job_submitted

## samples/batch/job_succeeded.json

- **Source AWS service**: AWS Batch
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/batch/latest/userguide/batch_cwe_events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, jobArn, jobId, jobDefinition, jobRoleArn, containerInstanceArn, taskArn
- **Test it drives**: parser/batch/batch_test.go::TestBatch/job_succeeded

## samples/beanstalk/aborted_operation.json

- **Source AWS service**: AWS Elastic Beanstalk
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/elasticbeanstalk/latest/dg/using-features.events.html + parsers/beanstalk.js inline text classifier
- **Captured on**: N/A (from docs)
- **Anonymization**: sns topic ARN, environment name
- **Test it drives**: parser/beanstalk/beanstalk_test.go::TestBeanstalk/aborted_operation

## samples/beanstalk/deploy_failed.json

- **Source AWS service**: AWS Elastic Beanstalk
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/elasticbeanstalk/latest/dg/using-features.events.html + parsers/beanstalk.js inline text classifier
- **Captured on**: N/A (from docs)
- **Anonymization**: sns topic ARN, environment name
- **Test it drives**: parser/beanstalk/beanstalk_test.go::TestBeanstalk/deploy_failed

## samples/beanstalk/removed_instance.json

- **Source AWS service**: AWS Elastic Beanstalk
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/elasticbeanstalk/latest/dg/using-features.events.html + parsers/beanstalk.js inline text classifier
- **Captured on**: N/A (from docs)
- **Anonymization**: sns topic ARN, environment name, InstanceId
- **Test it drives**: parser/beanstalk/beanstalk_test.go::TestBeanstalk/removed_instance

## samples/beanstalk/state_ok.json

- **Source AWS service**: AWS Elastic Beanstalk
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/elasticbeanstalk/latest/dg/using-features.events.html + parsers/beanstalk.js inline text classifier
- **Captured on**: N/A (from docs)
- **Anonymization**: sns topic ARN, environment name
- **Test it drives**: parser/beanstalk/beanstalk_test.go::TestBeanstalk/state_ok

## samples/beanstalk/state_red.json

- **Source AWS service**: AWS Elastic Beanstalk
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/elasticbeanstalk/latest/dg/using-features.events.html + parsers/beanstalk.js inline text classifier
- **Captured on**: N/A (from docs)
- **Anonymization**: sns topic ARN, environment name
- **Test it drives**: parser/beanstalk/beanstalk_test.go::TestBeanstalk/state_red

## samples/beanstalk/state_yellow.json

- **Source AWS service**: AWS Elastic Beanstalk
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/elasticbeanstalk/latest/dg/using-features.events.html + parsers/beanstalk.js inline text classifier
- **Captured on**: N/A (from docs)
- **Anonymization**: sns topic ARN, environment name
- **Test it drives**: parser/beanstalk/beanstalk_test.go::TestBeanstalk/state_yellow

## samples/cloudformation/malformed.json

- **Source AWS service**: AWS CloudFormation (malformed payload)
- **Delivery channel**: SNS topic
- **Provenance**: parsers/cloudformation.js line 20-23 (no LogicalResourceId/StackName → match returns false)
- **Captured on**: N/A (from docs)
- **Anonymization**: sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/malformed

## samples/cloudformation/resource_event_ignored.json

- **Source AWS service**: AWS CloudFormation (non-stack resource event)
- **Delivery channel**: SNS topic
- **Provenance**: parsers/cloudformation.js line 28-32 (LogicalResourceId != StackName → silent)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/resource_event_ignored

## samples/cloudformation/status_create_complete.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_create_complete

## samples/cloudformation/status_create_failed.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_create_failed

## samples/cloudformation/status_create_in_progress.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_create_in_progress

## samples/cloudformation/status_delete_complete.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_delete_complete

## samples/cloudformation/status_delete_failed.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_delete_failed

## samples/cloudformation/status_delete_in_progress.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_delete_in_progress

## samples/cloudformation/status_review_in_progress.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_review_in_progress

## samples/cloudformation/status_rollback_complete.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_rollback_complete

## samples/cloudformation/status_rollback_failed.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_rollback_failed

## samples/cloudformation/status_rollback_in_progress.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_rollback_in_progress

## samples/cloudformation/status_update_complete.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_update_complete

## samples/cloudformation/status_update_complete_cleanup_in_progress.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_update_complete_cleanup_in_progress

## samples/cloudformation/status_update_in_progress.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_update_in_progress

## samples/cloudformation/status_update_rollback_complete.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_update_rollback_complete

## samples/cloudformation/status_update_rollback_complete_cleanup_in_progress.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_update_rollback_complete_cleanup_in_progress

## samples/cloudformation/status_update_rollback_failed.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_update_rollback_failed

## samples/cloudformation/status_update_rollback_in_progress.json

- **Source AWS service**: AWS CloudFormation
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-monitor-stack.html + parsers/cloudformation.js statusMappings (line 66+)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, StackId UUID, sns topic ARN
- **Test it drives**: parser/cloudformation/cloudformation_test.go::TestCloudFormation/status_update_rollback_in_progress

## samples/cloudwatch/alarm_china.json

- **Source AWS service**: AWS CloudWatch Alarms (cn-* partition)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/AlarmThatSendsEmail.html + parsers/cloudwatch/chart.js resolveRegion()
- **Captured on**: N/A (from docs)
- **Anonymization**: AWSAccountId, AlarmArn, InstanceId, sns topic ARN, China partition retained
- **Test it drives**: parser/cloudwatch/cloudwatch_test.go::TestCloudWatch/alarm_china

## samples/cloudwatch/alarm_composite.json

- **Source AWS service**: AWS CloudWatch Composite Alarms
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Create_Composite_Alarm.html
- **Captured on**: N/A (from docs)
- **Anonymization**: AWSAccountId, child alarm ARNs, sns topic ARN
- **Test it drives**: parser/cloudwatch/cloudwatch_test.go::TestCloudWatch/alarm_composite

## samples/cloudwatch/alarm_critical_single_metric.json

- **Source AWS service**: AWS CloudWatch Alarms
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/AlarmThatSendsEmail.html#alarms-and-actions
- **Captured on**: N/A (from docs)
- **Anonymization**: AWSAccountId, AlarmArn, InstanceId, sns topic ARN
- **Test it drives**: parser/cloudwatch/cloudwatch_test.go::TestCloudWatch/alarm_critical_single_metric

## samples/cloudwatch/alarm_govcloud.json

- **Source AWS service**: AWS CloudWatch Alarms (us-gov-* partition)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/AlarmThatSendsEmail.html + Go-port partition table (§5b item 12)
- **Captured on**: N/A (from docs)
- **Anonymization**: AWSAccountId, AlarmArn, InstanceId, sns topic ARN, GovCloud partition retained
- **Test it drives**: parser/cloudwatch/cloudwatch_test.go::TestCloudWatch/alarm_govcloud

## samples/cloudwatch/alarm_lambda_with_log_link.json

- **Source AWS service**: AWS CloudWatch Alarms (AWS/Lambda)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/AlarmThatSendsEmail.html — Lambda-specific path inferred from parsers/cloudwatch/chart.js getCloudWatchUrl()
- **Captured on**: N/A (from docs)
- **Anonymization**: AWSAccountId, FunctionName preserved as 'aws-to-slack', sns topic ARN
- **Test it drives**: parser/cloudwatch/cloudwatch_test.go::TestCloudWatch/alarm_lambda_with_log_link

## samples/cloudwatch/alarm_metric_math.json

- **Source AWS service**: AWS CloudWatch Alarms (metric math)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/using-metric-math.html
- **Captured on**: N/A (from docs)
- **Anonymization**: AWSAccountId, AlarmArn, ALB id, sns topic ARN
- **Test it drives**: parser/cloudwatch/cloudwatch_test.go::TestCloudWatch/alarm_metric_math

## samples/cloudwatch/alarm_metric_math_alb_5xx.json

- **Source AWS service**: AWS CloudWatch Alarms (FILL math expr, ESA-1083 shape)
- **Delivery channel**: SNS topic
- **Provenance**: esai-infra ALB sparse-metric alarm pattern (ESA-1083) + CloudWatch FILL docs
- **Captured on**: N/A (from docs)
- **Anonymization**: AWSAccountId, AlarmArn, ALB id, sns topic ARN
- **Test it drives**: parser/cloudwatch/cloudwatch_test.go::TestCloudWatch/alarm_metric_math_alb_5xx

## samples/cloudwatch/alarm_ok.json

- **Source AWS service**: AWS CloudWatch Alarms
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/AlarmThatSendsEmail.html#alarms-and-actions
- **Captured on**: N/A (from docs)
- **Anonymization**: AWSAccountId, AlarmArn, InstanceId, sns topic ARN
- **Test it drives**: parser/cloudwatch/cloudwatch_test.go::TestCloudWatch/alarm_ok

## samples/cloudwatch/alarm_warning_insufficient_data.json

- **Source AWS service**: AWS CloudWatch Alarms
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/AlarmThatSendsEmail.html#alarms-and-actions
- **Captured on**: N/A (from docs)
- **Anonymization**: AWSAccountId, AlarmArn, InstanceId, sns topic ARN
- **Test it drives**: parser/cloudwatch/cloudwatch_test.go::TestCloudWatch/alarm_warning_insufficient_data

## samples/codebuild/build_failed.json

- **Source AWS service**: AWS CodeBuild (FAILED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codebuild/latest/userguide/sample-build-notifications.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, project ARN, build id, S3 artifact location
- **Test it drives**: parser/codebuild/codebuild_test.go::TestCodeBuild/build_failed

## samples/codebuild/build_in_progress.json

- **Source AWS service**: AWS CodeBuild (IN_PROGRESS)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codebuild/latest/userguide/sample-build-notifications.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, project ARN, build id, S3 artifact location
- **Test it drives**: parser/codebuild/codebuild_test.go::TestCodeBuild/build_in_progress

## samples/codebuild/build_phase_change.json

- **Source AWS service**: AWS CodeBuild (Build Phase Change — silenced by Go port)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codebuild/latest/userguide/sample-build-notifications.html ("CodeBuild Build Phase Change" detail-type) + §2 row 9 silence rule
- **Captured on**: N/A (from docs)
- **Anonymization**: account, project ARN, build id
- **Test it drives**: parser/codebuild/codebuild_test.go::TestCodeBuild/build_phase_change

## samples/codebuild/build_stopped.json

- **Source AWS service**: AWS CodeBuild (STOPPED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codebuild/latest/userguide/sample-build-notifications.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, project ARN, build id, S3 artifact location
- **Test it drives**: parser/codebuild/codebuild_test.go::TestCodeBuild/build_stopped

## samples/codebuild/build_succeeded.json

- **Source AWS service**: AWS CodeBuild (SUCCEEDED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codebuild/latest/userguide/sample-build-notifications.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, project ARN, build id, S3 artifact location
- **Test it drives**: parser/codebuild/codebuild_test.go::TestCodeBuild/build_succeeded

## samples/codecommit/pullrequest/pr_comment.json

- **Source AWS service**: AWS CodeCommit (Pull Request)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codecommit/latest/userguide/monitoring-events.html#pullRequestEvent + parsers/codecommit/pullrequest.js branch logic
- **Captured on**: N/A (from docs)
- **Anonymization**: account, repo ARN, callerUserArn, sourceCommit/destinationCommit SHAs
- **Test it drives**: parser/codecommit/codecommit_test.go::TestPullRequest/pr_comment

## samples/codecommit/pullrequest/pr_created.json

- **Source AWS service**: AWS CodeCommit (Pull Request)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codecommit/latest/userguide/monitoring-events.html#pullRequestEvent + parsers/codecommit/pullrequest.js branch logic
- **Captured on**: N/A (from docs)
- **Anonymization**: account, repo ARN, callerUserArn, sourceCommit/destinationCommit SHAs
- **Test it drives**: parser/codecommit/codecommit_test.go::TestPullRequest/pr_created

## samples/codecommit/pullrequest/pr_merged.json

- **Source AWS service**: AWS CodeCommit (Pull Request)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codecommit/latest/userguide/monitoring-events.html#pullRequestEvent + parsers/codecommit/pullrequest.js branch logic
- **Captured on**: N/A (from docs)
- **Anonymization**: account, repo ARN, callerUserArn, sourceCommit/destinationCommit SHAs
- **Test it drives**: parser/codecommit/codecommit_test.go::TestPullRequest/pr_merged

## samples/codecommit/pullrequest/pr_source_updated.json

- **Source AWS service**: AWS CodeCommit (Pull Request)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codecommit/latest/userguide/monitoring-events.html#pullRequestEvent + parsers/codecommit/pullrequest.js branch logic
- **Captured on**: N/A (from docs)
- **Anonymization**: account, repo ARN, callerUserArn, sourceCommit/destinationCommit SHAs
- **Test it drives**: parser/codecommit/codecommit_test.go::TestPullRequest/pr_source_updated

## samples/codecommit/pullrequest/pr_status_closed_unmerged.json

- **Source AWS service**: AWS CodeCommit (Pull Request)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codecommit/latest/userguide/monitoring-events.html#pullRequestEvent + parsers/codecommit/pullrequest.js branch logic
- **Captured on**: N/A (from docs)
- **Anonymization**: account, repo ARN, callerUserArn, sourceCommit/destinationCommit SHAs
- **Test it drives**: parser/codecommit/codecommit_test.go::TestPullRequest/pr_status_closed_unmerged

## samples/codecommit/repository/ref_created_branch.json

- **Source AWS service**: AWS CodeCommit (Repository)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codecommit/latest/userguide/monitoring-events.html#referenceEvent
- **Captured on**: N/A (from docs)
- **Anonymization**: account, repo id, callerUserArn, commit SHAs
- **Test it drives**: parser/codecommit/codecommit_test.go::TestRepository/ref_created_branch

## samples/codecommit/repository/ref_created_tag.json

- **Source AWS service**: AWS CodeCommit (Repository)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codecommit/latest/userguide/monitoring-events.html#referenceEvent
- **Captured on**: N/A (from docs)
- **Anonymization**: account, repo id, callerUserArn, commit SHAs
- **Test it drives**: parser/codecommit/codecommit_test.go::TestRepository/ref_created_tag

## samples/codecommit/repository/ref_deleted_branch.json

- **Source AWS service**: AWS CodeCommit (Repository)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codecommit/latest/userguide/monitoring-events.html#referenceEvent
- **Captured on**: N/A (from docs)
- **Anonymization**: account, repo id, callerUserArn, commit SHAs
- **Test it drives**: parser/codecommit/codecommit_test.go::TestRepository/ref_deleted_branch

## samples/codecommit/repository/ref_deleted_tag.json

- **Source AWS service**: AWS CodeCommit (Repository)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codecommit/latest/userguide/monitoring-events.html#referenceEvent
- **Captured on**: N/A (from docs)
- **Anonymization**: account, repo id, callerUserArn, commit SHAs
- **Test it drives**: parser/codecommit/codecommit_test.go::TestRepository/ref_deleted_tag

## samples/codecommit/repository/ref_updated_branch.json

- **Source AWS service**: AWS CodeCommit (Repository)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codecommit/latest/userguide/monitoring-events.html#referenceEvent
- **Captured on**: N/A (from docs)
- **Anonymization**: account, repo id, callerUserArn, commit SHAs
- **Test it drives**: parser/codecommit/codecommit_test.go::TestRepository/ref_updated_branch

## samples/codedeploy/eventbridge/failure.json

- **Source AWS service**: AWS CodeDeploy (FAILURE)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codedeploy/latest/userguide/monitoring-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, application ARN, deploymentId
- **Test it drives**: parser/codedeploy/codedeploy_test.go::TestEventBridge/failure

## samples/codedeploy/eventbridge/ready.json

- **Source AWS service**: AWS CodeDeploy (READY)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codedeploy/latest/userguide/monitoring-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, application ARN, deploymentId
- **Test it drives**: parser/codedeploy/codedeploy_test.go::TestEventBridge/ready

## samples/codedeploy/eventbridge/start.json

- **Source AWS service**: AWS CodeDeploy (START)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codedeploy/latest/userguide/monitoring-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, application ARN, deploymentId
- **Test it drives**: parser/codedeploy/codedeploy_test.go::TestEventBridge/start

## samples/codedeploy/eventbridge/stop.json

- **Source AWS service**: AWS CodeDeploy (STOP)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codedeploy/latest/userguide/monitoring-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, application ARN, deploymentId
- **Test it drives**: parser/codedeploy/codedeploy_test.go::TestEventBridge/stop

## samples/codedeploy/eventbridge/success.json

- **Source AWS service**: AWS CodeDeploy (SUCCESS)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codedeploy/latest/userguide/monitoring-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, application ARN, deploymentId
- **Test it drives**: parser/codedeploy/codedeploy_test.go::TestEventBridge/success

## samples/codedeploy/sns/created.json

- **Source AWS service**: AWS CodeDeploy (CREATED)
- **Delivery channel**: SNS topic (CodeDeploy trigger)
- **Provenance**: https://docs.aws.amazon.com/codedeploy/latest/userguide/monitoring-sns-event-notifications-json-content.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, deploymentId, sns topic ARN
- **Test it drives**: parser/codedeploy/codedeploy_test.go::TestSNS/created

## samples/codedeploy/sns/failed.json

- **Source AWS service**: AWS CodeDeploy (FAILED)
- **Delivery channel**: SNS topic (CodeDeploy trigger)
- **Provenance**: https://docs.aws.amazon.com/codedeploy/latest/userguide/monitoring-sns-event-notifications-json-content.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, deploymentId, sns topic ARN
- **Test it drives**: parser/codedeploy/codedeploy_test.go::TestSNS/failed

## samples/codedeploy/sns/ready.json

- **Source AWS service**: AWS CodeDeploy (READY)
- **Delivery channel**: SNS topic (CodeDeploy trigger)
- **Provenance**: https://docs.aws.amazon.com/codedeploy/latest/userguide/monitoring-sns-event-notifications-json-content.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, deploymentId, sns topic ARN
- **Test it drives**: parser/codedeploy/codedeploy_test.go::TestSNS/ready

## samples/codedeploy/sns/stopped.json

- **Source AWS service**: AWS CodeDeploy (STOPPED)
- **Delivery channel**: SNS topic (CodeDeploy trigger)
- **Provenance**: https://docs.aws.amazon.com/codedeploy/latest/userguide/monitoring-sns-event-notifications-json-content.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, deploymentId, sns topic ARN
- **Test it drives**: parser/codedeploy/codedeploy_test.go::TestSNS/stopped

## samples/codedeploy/sns/succeeded.json

- **Source AWS service**: AWS CodeDeploy (SUCCEEDED)
- **Delivery channel**: SNS topic (CodeDeploy trigger)
- **Provenance**: https://docs.aws.amazon.com/codedeploy/latest/userguide/monitoring-sns-event-notifications-json-content.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, deploymentId, sns topic ARN
- **Test it drives**: parser/codedeploy/codedeploy_test.go::TestSNS/succeeded

## samples/codepipeline/action_failed.json

- **Source AWS service**: AWS CodePipeline Action Execution (FAILED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/detect-state-changes-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline ARN, execution-id
- **Test it drives**: parser/codepipeline/codepipeline_test.go::TestPipeline/action_failed

## samples/codepipeline/action_started.json

- **Source AWS service**: AWS CodePipeline Action Execution (STARTED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/detect-state-changes-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline ARN, execution-id
- **Test it drives**: parser/codepipeline/codepipeline_test.go::TestPipeline/action_started

## samples/codepipeline/action_succeeded.json

- **Source AWS service**: AWS CodePipeline Action Execution (SUCCEEDED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/detect-state-changes-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline ARN, execution-id
- **Test it drives**: parser/codepipeline/codepipeline_test.go::TestPipeline/action_succeeded

## samples/codepipeline/approval/approval_days_to_expiry.json

- **Source AWS service**: AWS CodePipeline Manual Approval (numHours >= 40)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/approvals-json-format.html + parsers/codepipeline-approval.js line 20-33
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline name, sns topic ARN, approval token
- **Test it drives**: parser/codepipeline/approval_test.go::TestApproval/approval_days_to_expiry

## samples/codepipeline/approval/approval_hours_to_expiry.json

- **Source AWS service**: AWS CodePipeline Manual Approval (numHours < 40)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/approvals-json-format.html + parsers/codepipeline-approval.js line 20-33
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline name, sns topic ARN, approval token
- **Test it drives**: parser/codepipeline/approval_test.go::TestApproval/approval_hours_to_expiry

## samples/codepipeline/approval/approval_minutes_to_expiry.json

- **Source AWS service**: AWS CodePipeline Manual Approval (numHours < 1)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/approvals-json-format.html + parsers/codepipeline-approval.js line 20-33
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline name, sns topic ARN, approval token
- **Test it drives**: parser/codepipeline/approval_test.go::TestApproval/approval_minutes_to_expiry

## samples/codepipeline/approval/approval_past_expiry.json

- **Source AWS service**: AWS CodePipeline Manual Approval (numHours < 0.001)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/approvals-json-format.html + parsers/codepipeline-approval.js line 20-33
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline name, sns topic ARN, approval token
- **Test it drives**: parser/codepipeline/approval_test.go::TestApproval/approval_past_expiry

## samples/codepipeline/approval/approval_with_custom_data.json

- **Source AWS service**: AWS CodePipeline Manual Approval (with customData)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/approvals-json-format.html + parsers/codepipeline-approval.js customData branch (line 16, 36-38)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline name, sns topic ARN, approval token
- **Test it drives**: parser/codepipeline/approval_test.go::TestApproval/with_custom_data

## samples/codepipeline/pipeline_canceled.json

- **Source AWS service**: AWS CodePipeline Pipeline Execution (CANCELED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/detect-state-changes-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline ARN, execution-id
- **Test it drives**: parser/codepipeline/codepipeline_test.go::TestPipeline/pipeline_canceled

## samples/codepipeline/pipeline_failed.json

- **Source AWS service**: AWS CodePipeline Pipeline Execution (FAILED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/detect-state-changes-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline ARN, execution-id
- **Test it drives**: parser/codepipeline/codepipeline_test.go::TestPipeline/pipeline_failed

## samples/codepipeline/pipeline_resumed.json

- **Source AWS service**: AWS CodePipeline Pipeline Execution (RESUMED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/detect-state-changes-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline ARN, execution-id
- **Test it drives**: parser/codepipeline/codepipeline_test.go::TestPipeline/pipeline_resumed

## samples/codepipeline/pipeline_started.json

- **Source AWS service**: AWS CodePipeline Pipeline Execution (STARTED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/detect-state-changes-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline ARN, execution-id
- **Test it drives**: parser/codepipeline/codepipeline_test.go::TestPipeline/pipeline_started

## samples/codepipeline/pipeline_stopped.json

- **Source AWS service**: AWS CodePipeline Pipeline Execution (STOPPED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/detect-state-changes-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline ARN, execution-id
- **Test it drives**: parser/codepipeline/codepipeline_test.go::TestPipeline/pipeline_stopped

## samples/codepipeline/pipeline_stopping.json

- **Source AWS service**: AWS CodePipeline Pipeline Execution (STOPPING)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/detect-state-changes-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline ARN, execution-id
- **Test it drives**: parser/codepipeline/codepipeline_test.go::TestPipeline/pipeline_stopping

## samples/codepipeline/pipeline_succeeded.json

- **Source AWS service**: AWS CodePipeline Pipeline Execution (SUCCEEDED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/detect-state-changes-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline ARN, execution-id
- **Test it drives**: parser/codepipeline/codepipeline_test.go::TestPipeline/pipeline_succeeded

## samples/codepipeline/pipeline_superseded.json

- **Source AWS service**: AWS CodePipeline Pipeline Execution (SUPERSEDED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/detect-state-changes-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline ARN, execution-id
- **Test it drives**: parser/codepipeline/codepipeline_test.go::TestPipeline/pipeline_superseded

## samples/codepipeline/stage_failed.json

- **Source AWS service**: AWS CodePipeline Stage Execution (FAILED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/detect-state-changes-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline ARN, execution-id
- **Test it drives**: parser/codepipeline/codepipeline_test.go::TestPipeline/stage_failed

## samples/codepipeline/stage_started.json

- **Source AWS service**: AWS CodePipeline Stage Execution (STARTED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/detect-state-changes-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline ARN, execution-id
- **Test it drives**: parser/codepipeline/codepipeline_test.go::TestPipeline/stage_started

## samples/codepipeline/stage_succeeded.json

- **Source AWS service**: AWS CodePipeline Stage Execution (SUCCEEDED)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/codepipeline/latest/userguide/detect-state-changes-cloudwatch-events.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, pipeline ARN, execution-id
- **Test it drives**: parser/codepipeline/codepipeline_test.go::TestPipeline/stage_succeeded

## samples/ecs/container_instance_state_change.json

- **Source AWS service**: AWS ECS (ECS Container Instance State Change (fall-through per §2 row 18))
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_cwe_events.html + parsers/ecs-event.js inline JSDoc examples (line 22-80, 96-)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, cluster ARN, task ARN, container ARN, ECR image account
- **Test it drives**: parser/ecs/ecs_test.go::TestEcs/container_instance_state_change

## samples/ecs/deployment_state_change.json

- **Source AWS service**: AWS ECS (ECS Deployment State Change (fall-through per §2 row 18))
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_cwe_events.html + parsers/ecs-event.js inline JSDoc examples (line 22-80, 96-)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, cluster ARN, task ARN, container ARN, ECR image account
- **Test it drives**: parser/ecs/ecs_test.go::TestEcs/deployment_state_change

## samples/ecs/service_action_deployment_failed.json

- **Source AWS service**: AWS ECS (ECS Service Action (ERROR/SERVICE_DEPLOYMENT_FAILED))
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_cwe_events.html + parsers/ecs-event.js inline JSDoc examples (line 22-80, 96-)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, cluster ARN, task ARN, container ARN, ECR image account
- **Test it drives**: parser/ecs/ecs_test.go::TestEcs/service_action_deployment_failed

## samples/ecs/service_action_steady_state.json

- **Source AWS service**: AWS ECS (ECS Service Action (INFO/SERVICE_STEADY_STATE))
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_cwe_events.html + parsers/ecs-event.js inline JSDoc examples (line 22-80, 96-)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, cluster ARN, task ARN, container ARN, ECR image account
- **Test it drives**: parser/ecs/ecs_test.go::TestEcs/service_action_steady_state

## samples/ecs/service_action_task_placement_failure.json

- **Source AWS service**: AWS ECS (ECS Service Action (WARN/SERVICE_TASK_PLACEMENT_FAILURE))
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_cwe_events.html + parsers/ecs-event.js inline JSDoc examples (line 22-80, 96-)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, cluster ARN, task ARN, container ARN, ECR image account
- **Test it drives**: parser/ecs/ecs_test.go::TestEcs/service_action_task_placement_failure

## samples/ecs/task_state_pending.json

- **Source AWS service**: AWS ECS (ECS Task State Change (PROVISIONING/PENDING))
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_cwe_events.html + parsers/ecs-event.js inline JSDoc examples (line 22-80, 96-)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, cluster ARN, task ARN, container ARN, ECR image account
- **Test it drives**: parser/ecs/ecs_test.go::TestEcs/task_state_pending

## samples/ecs/task_state_running.json

- **Source AWS service**: AWS ECS (ECS Task State Change (RUNNING))
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_cwe_events.html + parsers/ecs-event.js inline JSDoc examples (line 22-80, 96-)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, cluster ARN, task ARN, container ARN, ECR image account
- **Test it drives**: parser/ecs/ecs_test.go::TestEcs/task_state_running

## samples/ecs/task_state_stopped.json

- **Source AWS service**: AWS ECS (ECS Task State Change (STOPPED with stoppedReason))
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_cwe_events.html + parsers/ecs-event.js inline JSDoc examples (line 22-80, 96-)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, cluster ARN, task ARN, container ARN, ECR image account
- **Test it drives**: parser/ecs/ecs_test.go::TestEcs/task_state_stopped

## samples/generic/eventbridge_unknown_source.json

- **Source AWS service**: Generic fallback (custom EventBridge source)
- **Delivery channel**: EventBridge default bus
- **Provenance**: parsers/generic.js — fallback handler for unmatched EventBridge events
- **Captured on**: N/A (from docs)
- **Anonymization**: account
- **Test it drives**: parser/generic/generic_test.go::TestGeneric/eventbridge_unknown_source

## samples/generic/sns_unknown_topic.json

- **Source AWS service**: Generic fallback (unknown SNS topic)
- **Delivery channel**: SNS topic
- **Provenance**: parsers/generic.js — fallback handler for unmatched SNS topic events
- **Captured on**: N/A (from docs)
- **Anonymization**: sns topic ARN
- **Test it drives**: parser/generic/generic_test.go::TestGeneric/sns_unknown_topic

## samples/guardduty/aws_api_call.json

- **Source AWS service**: Amazon GuardDuty (AWS_API_CALL actionType)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/guardduty/latest/ug/guardduty_finding-format.html + https://github.com/aws-samples/amazon-guardduty-tester (finding samples)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, detectorId, finding id, InstanceId, AKIA*, AROAJ*, IP → 203.0.113.x
- **Test it drives**: parser/guardduty/guardduty_test.go::TestGuardDuty/aws_api_call

## samples/guardduty/dns_request.json

- **Source AWS service**: Amazon GuardDuty (DNS_REQUEST actionType)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/guardduty/latest/ug/guardduty_finding-format.html + https://github.com/aws-samples/amazon-guardduty-tester (finding samples)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, detectorId, finding id, InstanceId, AKIA*, AROAJ*, IP → 203.0.113.x
- **Test it drives**: parser/guardduty/guardduty_test.go::TestGuardDuty/dns_request

## samples/guardduty/kubernetes_api_call.json

- **Source AWS service**: Amazon GuardDuty (KUBERNETES_API_CALL actionType)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/guardduty/latest/ug/guardduty_finding-format.html + https://github.com/aws-samples/amazon-guardduty-tester (finding samples)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, detectorId, finding id, InstanceId, AKIA*, AROAJ*, IP → 203.0.113.x
- **Test it drives**: parser/guardduty/guardduty_test.go::TestGuardDuty/kubernetes_api_call

## samples/guardduty/network_connection.json

- **Source AWS service**: Amazon GuardDuty (NETWORK_CONNECTION actionType)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/guardduty/latest/ug/guardduty_finding-format.html + https://github.com/aws-samples/amazon-guardduty-tester (finding samples)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, detectorId, finding id, InstanceId, AKIA*, AROAJ*, IP → 203.0.113.x
- **Test it drives**: parser/guardduty/guardduty_test.go::TestGuardDuty/network_connection

## samples/guardduty/port_probe.json

- **Source AWS service**: Amazon GuardDuty (PORT_PROBE actionType)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/guardduty/latest/ug/guardduty_finding-format.html + https://github.com/aws-samples/amazon-guardduty-tester (finding samples)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, detectorId, finding id, InstanceId, AKIA*, AROAJ*, IP → 203.0.113.x
- **Test it drives**: parser/guardduty/guardduty_test.go::TestGuardDuty/port_probe

## samples/guardduty/rds_login_attempt.json

- **Source AWS service**: Amazon GuardDuty (RDS_LOGIN_ATTEMPT actionType)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/guardduty/latest/ug/guardduty_finding-format.html + https://github.com/aws-samples/amazon-guardduty-tester (finding samples)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, detectorId, finding id, InstanceId, AKIA*, AROAJ*, IP → 203.0.113.x
- **Test it drives**: parser/guardduty/guardduty_test.go::TestGuardDuty/rds_login_attempt

## samples/guardduty/resource_access_key.json

- **Source AWS service**: Amazon GuardDuty (AccessKey resourceType)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/guardduty/latest/ug/guardduty_finding-format.html + https://github.com/aws-samples/amazon-guardduty-tester (finding samples)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, detectorId, finding id, InstanceId, AKIA*, AROAJ*, IP → 203.0.113.x
- **Test it drives**: parser/guardduty/guardduty_test.go::TestGuardDuty/resource_access_key

## samples/guardduty/resource_eks.json

- **Source AWS service**: Amazon GuardDuty (EKSCluster resourceType)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/guardduty/latest/ug/guardduty_finding-format.html + https://github.com/aws-samples/amazon-guardduty-tester (finding samples)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, detectorId, finding id, InstanceId, AKIA*, AROAJ*, IP → 203.0.113.x
- **Test it drives**: parser/guardduty/guardduty_test.go::TestGuardDuty/resource_eks

## samples/guardduty/resource_instance.json

- **Source AWS service**: Amazon GuardDuty (Instance resourceType)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/guardduty/latest/ug/guardduty_finding-format.html + https://github.com/aws-samples/amazon-guardduty-tester (finding samples)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, detectorId, finding id, InstanceId, AKIA*, AROAJ*, IP → 203.0.113.x
- **Test it drives**: parser/guardduty/guardduty_test.go::TestGuardDuty/resource_instance

## samples/guardduty/resource_s3_bucket.json

- **Source AWS service**: Amazon GuardDuty (S3Bucket resourceType)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/guardduty/latest/ug/guardduty_finding-format.html + https://github.com/aws-samples/amazon-guardduty-tester (finding samples)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, detectorId, finding id, InstanceId, AKIA*, AROAJ*, IP → 203.0.113.x
- **Test it drives**: parser/guardduty/guardduty_test.go::TestGuardDuty/resource_s3_bucket

## samples/guardduty/severity_high.json

- **Source AWS service**: Amazon GuardDuty (severity 8 (high))
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/guardduty/latest/ug/guardduty_finding-format.html + https://github.com/aws-samples/amazon-guardduty-tester (finding samples)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, detectorId, finding id, InstanceId, AKIA*, AROAJ*, IP → 203.0.113.x
- **Test it drives**: parser/guardduty/guardduty_test.go::TestGuardDuty/severity_high

## samples/guardduty/severity_low.json

- **Source AWS service**: Amazon GuardDuty (severity 2 (low))
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/guardduty/latest/ug/guardduty_finding-format.html + https://github.com/aws-samples/amazon-guardduty-tester (finding samples)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, detectorId, finding id, InstanceId, AKIA*, AROAJ*, IP → 203.0.113.x
- **Test it drives**: parser/guardduty/guardduty_test.go::TestGuardDuty/severity_low

## samples/guardduty/severity_medium.json

- **Source AWS service**: Amazon GuardDuty (severity 5 (medium))
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/guardduty/latest/ug/guardduty_finding-format.html + https://github.com/aws-samples/amazon-guardduty-tester (finding samples)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, detectorId, finding id, InstanceId, AKIA*, AROAJ*, IP → 203.0.113.x
- **Test it drives**: parser/guardduty/guardduty_test.go::TestGuardDuty/severity_medium

## samples/inspector/classic/assessment_run_completed.json

- **Source AWS service**: Amazon Inspector classic (ASSESSMENT_RUN_COMPLETED (with findingsCount))
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/inspector/v1/userguide/inspector_assessments.html#inspector_assessments-monitor + parsers/inspector.js event switch (line 86-133)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, template/run ARNs, sns topic ARN
- **Test it drives**: parser/inspector/inspector_test.go::TestInspectorClassic/assessment_run_completed

## samples/inspector/classic/assessment_run_started.json

- **Source AWS service**: Amazon Inspector classic (ASSESSMENT_RUN_STARTED)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/inspector/v1/userguide/inspector_assessments.html#inspector_assessments-monitor + parsers/inspector.js event switch (line 86-133)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, template/run ARNs, sns topic ARN
- **Test it drives**: parser/inspector/inspector_test.go::TestInspectorClassic/assessment_run_started

## samples/inspector/classic/assessment_run_state_changed.json

- **Source AWS service**: Amazon Inspector classic (ASSESSMENT_RUN_STATE_CHANGED (COLLECTING_DATA))
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/inspector/v1/userguide/inspector_assessments.html#inspector_assessments-monitor + parsers/inspector.js event switch (line 86-133)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, template/run ARNs, sns topic ARN
- **Test it drives**: parser/inspector/inspector_test.go::TestInspectorClassic/assessment_run_state_changed

## samples/inspector/classic/enable_assessment_notifications.json

- **Source AWS service**: Amazon Inspector classic (ENABLE_ASSESSMENT_NOTIFICATIONS (silenced))
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/inspector/v1/userguide/inspector_assessments.html#inspector_assessments-monitor + parsers/inspector.js event switch (line 86-133)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, template/run ARNs, sns topic ARN
- **Test it drives**: parser/inspector/inspector_test.go::TestInspectorClassic/enable_assessment_notifications

## samples/inspector/classic/finding_reported.json

- **Source AWS service**: Amazon Inspector classic (FINDING_REPORTED)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/inspector/v1/userguide/inspector_assessments.html#inspector_assessments-monitor + parsers/inspector.js event switch (line 86-133)
- **Captured on**: N/A (from docs)
- **Anonymization**: account, template/run ARNs, sns topic ARN
- **Test it drives**: parser/inspector/inspector_test.go::TestInspectorClassic/finding_reported

## samples/inspector2/finding_critical_ecr.json

- **Source AWS service**: Amazon Inspector (CRITICAL severity, AWS_ECR_CONTAINER_IMAGE resource)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/inspector/latest/user/eventbridge-integration.html + parsers/inspector2.js
- **Captured on**: N/A (from docs)
- **Anonymization**: account, findingArn, ECR repository name, Lambda functionArn, InstanceId, imageHash, imageId
- **Test it drives**: parser/inspector2/inspector2_test.go::TestInspector2/finding_critical_ecr

## samples/inspector2/finding_high_ec2.json

- **Source AWS service**: Amazon Inspector (HIGH severity, AWS_EC2_INSTANCE resource)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/inspector/latest/user/eventbridge-integration.html + parsers/inspector2.js
- **Captured on**: N/A (from docs)
- **Anonymization**: account, findingArn, ECR repository name, Lambda functionArn, InstanceId, imageHash, imageId
- **Test it drives**: parser/inspector2/inspector2_test.go::TestInspector2/finding_high_ec2

## samples/inspector2/finding_high_lambda.json

- **Source AWS service**: Amazon Inspector (HIGH severity, AWS_LAMBDA_FUNCTION resource)
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/inspector/latest/user/eventbridge-integration.html + parsers/inspector2.js
- **Captured on**: N/A (from docs)
- **Anonymization**: account, findingArn, ECR repository name, Lambda functionArn, InstanceId, imageHash, imageId
- **Test it drives**: parser/inspector2/inspector2_test.go::TestInspector2/finding_high_lambda

## samples/inspector2/finding_medium_silenced.json

- **Source AWS service**: Amazon Inspector (MEDIUM severity (silenced by parser, exercises filter))
- **Delivery channel**: EventBridge default bus
- **Provenance**: https://docs.aws.amazon.com/inspector/latest/user/eventbridge-integration.html + parsers/inspector2.js
- **Captured on**: N/A (from docs)
- **Anonymization**: account, findingArn, ECR repository name, Lambda functionArn, InstanceId, imageHash, imageId
- **Test it drives**: parser/inspector2/inspector2_test.go::TestInspector2/finding_medium_silenced

## samples/rds/availability.json

- **Source AWS service**: Amazon RDS event subscription (category: availability)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ListEvents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, DB instance name, sns topic ARN
- **Test it drives**: parser/rds/rds_test.go::TestRDS/availability

## samples/rds/backup.json

- **Source AWS service**: Amazon RDS event subscription (category: backup)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ListEvents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, DB instance name, sns topic ARN
- **Test it drives**: parser/rds/rds_test.go::TestRDS/backup

## samples/rds/configuration_change.json

- **Source AWS service**: Amazon RDS event subscription (category: configuration change)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ListEvents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, DB instance name, sns topic ARN
- **Test it drives**: parser/rds/rds_test.go::TestRDS/configuration_change

## samples/rds/creation.json

- **Source AWS service**: Amazon RDS event subscription (category: creation)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ListEvents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, DB instance name, sns topic ARN
- **Test it drives**: parser/rds/rds_test.go::TestRDS/creation

## samples/rds/deletion.json

- **Source AWS service**: Amazon RDS event subscription (category: deletion)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ListEvents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, DB instance name, sns topic ARN
- **Test it drives**: parser/rds/rds_test.go::TestRDS/deletion

## samples/rds/failover.json

- **Source AWS service**: Amazon RDS event subscription (category: failover)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ListEvents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, DB instance name, sns topic ARN
- **Test it drives**: parser/rds/rds_test.go::TestRDS/failover

## samples/rds/failure.json

- **Source AWS service**: Amazon RDS event subscription (category: failure)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ListEvents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, DB instance name, sns topic ARN
- **Test it drives**: parser/rds/rds_test.go::TestRDS/failure

## samples/rds/low_storage.json

- **Source AWS service**: Amazon RDS event subscription (category: low storage)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ListEvents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, DB instance name, sns topic ARN
- **Test it drives**: parser/rds/rds_test.go::TestRDS/low_storage

## samples/rds/maintenance.json

- **Source AWS service**: Amazon RDS event subscription (category: maintenance)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ListEvents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, DB instance name, sns topic ARN
- **Test it drives**: parser/rds/rds_test.go::TestRDS/maintenance

## samples/rds/notification.json

- **Source AWS service**: Amazon RDS event subscription (category: notification)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ListEvents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, DB instance name, sns topic ARN
- **Test it drives**: parser/rds/rds_test.go::TestRDS/notification

## samples/rds/read_replica.json

- **Source AWS service**: Amazon RDS event subscription (category: read replica)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ListEvents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, DB instance name, sns topic ARN
- **Test it drives**: parser/rds/rds_test.go::TestRDS/read_replica

## samples/rds/recovery.json

- **Source AWS service**: Amazon RDS event subscription (category: recovery)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ListEvents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, DB instance name, sns topic ARN
- **Test it drives**: parser/rds/rds_test.go::TestRDS/recovery

## samples/rds/restoration.json

- **Source AWS service**: Amazon RDS event subscription (category: restoration)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ListEvents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, DB instance name, sns topic ARN
- **Test it drives**: parser/rds/rds_test.go::TestRDS/restoration

## samples/rds/security_patching.json

- **Source AWS service**: Amazon RDS event subscription (category: security patching)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ListEvents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, DB instance name, sns topic ARN
- **Test it drives**: parser/rds/rds_test.go::TestRDS/security_patching

## samples/ses/bounce/permanent_general.json

- **Source AWS service**: Amazon SES bounce (permanent general)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#bounce-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/bounce_test.go::TestBounce/permanent_general

## samples/ses/bounce/permanent_no_email.json

- **Source AWS service**: Amazon SES bounce (permanent no email)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#bounce-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/bounce_test.go::TestBounce/permanent_no_email

## samples/ses/bounce/permanent_on_account_suppression_list.json

- **Source AWS service**: Amazon SES bounce (permanent on account suppression list)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#bounce-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/bounce_test.go::TestBounce/permanent_on_account_suppression_list

## samples/ses/bounce/permanent_suppressed.json

- **Source AWS service**: Amazon SES bounce (permanent suppressed)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#bounce-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/bounce_test.go::TestBounce/permanent_suppressed

## samples/ses/bounce/transient_attachment_rejected.json

- **Source AWS service**: Amazon SES bounce (transient attachment rejected)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#bounce-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/bounce_test.go::TestBounce/transient_attachment_rejected

## samples/ses/bounce/transient_content_rejected.json

- **Source AWS service**: Amazon SES bounce (transient content rejected)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#bounce-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/bounce_test.go::TestBounce/transient_content_rejected

## samples/ses/bounce/transient_general.json

- **Source AWS service**: Amazon SES bounce (transient general)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#bounce-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/bounce_test.go::TestBounce/transient_general

## samples/ses/bounce/transient_mailbox_full.json

- **Source AWS service**: Amazon SES bounce (transient mailbox full)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#bounce-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/bounce_test.go::TestBounce/transient_mailbox_full

## samples/ses/bounce/transient_message_too_large.json

- **Source AWS service**: Amazon SES bounce (transient message too large)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#bounce-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/bounce_test.go::TestBounce/transient_message_too_large

## samples/ses/bounce/undetermined.json

- **Source AWS service**: Amazon SES bounce (undetermined)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#bounce-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/bounce_test.go::TestBounce/undetermined

## samples/ses/complaint/abuse.json

- **Source AWS service**: Amazon SES complaint (abuse)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#complaint-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/complaint_test.go::TestComplaint/abuse

## samples/ses/complaint/auth_failure.json

- **Source AWS service**: Amazon SES complaint (auth-failure)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#complaint-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/complaint_test.go::TestComplaint/auth_failure

## samples/ses/complaint/fraud.json

- **Source AWS service**: Amazon SES complaint (fraud)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#complaint-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/complaint_test.go::TestComplaint/fraud

## samples/ses/complaint/not_spam.json

- **Source AWS service**: Amazon SES complaint (not-spam)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#complaint-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/complaint_test.go::TestComplaint/not_spam

## samples/ses/complaint/other.json

- **Source AWS service**: Amazon SES complaint (other)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#complaint-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/complaint_test.go::TestComplaint/other

## samples/ses/complaint/virus.json

- **Source AWS service**: Amazon SES complaint (virus)
- **Delivery channel**: SNS topic
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/notification-contents.html#complaint-object
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, source IP, sns topic ARN, feedbackId
- **Test it drives**: parser/ses/complaint_test.go::TestComplaint/virus

## samples/ses/received/multi_recipient.json

- **Source AWS service**: Amazon SES received (multiple destination addresses)
- **Delivery channel**: SNS topic (SES inbound rule action)
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/receiving-email-notifications-contents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, sns topic ARN, messageId
- **Test it drives**: parser/ses/received_test.go::TestReceived/multi_recipient

## samples/ses/received/simple.json

- **Source AWS service**: Amazon SES received (plain-text body)
- **Delivery channel**: SNS topic (SES inbound rule action)
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/receiving-email-notifications-contents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, sns topic ARN, messageId
- **Test it drives**: parser/ses/received_test.go::TestReceived/simple

## samples/ses/received/with_attachments.json

- **Source AWS service**: Amazon SES received (multipart/mixed body with attachment)
- **Delivery channel**: SNS topic (SES inbound rule action)
- **Provenance**: https://docs.aws.amazon.com/ses/latest/dg/receiving-email-notifications-contents.html
- **Captured on**: N/A (from docs)
- **Anonymization**: account, recipient/sender emails, sns topic ARN, messageId
- **Test it drives**: parser/ses/received_test.go::TestReceived/with_attachments

## samples/sns_envelope/multi_record.json

- **Source AWS service**: AWS Lambda runtime (SNS source)
- **Delivery channel**: Lambda direct (SNS-Lambda invocation event with N Records)
- **Provenance**: https://docs.aws.amazon.com/lambda/latest/dg/with-sns.html — extended to two Records
- **Captured on**: N/A (from docs)
- **Anonymization**: account ID in TopicArn, MessageIds
- **Test it drives**: envelope/envelope_test.go::TestEnvelope/multi_record

## samples/sns_envelope/record_with_json_string.json

- **Source AWS service**: AWS Lambda runtime (SNS source)
- **Delivery channel**: Lambda direct (SNS message body is JSON-encoded string)
- **Provenance**: https://docs.aws.amazon.com/lambda/latest/dg/with-sns.html — Message field is a JSON-encoded string per SNS contract
- **Captured on**: N/A (from docs)
- **Anonymization**: account ID in TopicArn, MessageId
- **Test it drives**: envelope/envelope_test.go::TestEnvelope/record_with_json_string

## samples/sns_envelope/record_with_plain_text.json

- **Source AWS service**: AWS Lambda runtime (SNS source)
- **Delivery channel**: Lambda direct (SNS message body is plain text, e.g. CloudFormation)
- **Provenance**: https://docs.aws.amazon.com/lambda/latest/dg/with-sns.html + parsers/cloudformation.js (line-delimited body shape)
- **Captured on**: N/A (from docs)
- **Anonymization**: account ID in TopicArn, MessageId
- **Test it drives**: envelope/envelope_test.go::TestEnvelope/record_with_plain_text

## samples/sns_envelope/single_record.json

- **Source AWS service**: AWS Lambda runtime (SNS source)
- **Delivery channel**: Lambda direct (SNS-Lambda invocation event)
- **Provenance**: https://docs.aws.amazon.com/lambda/latest/dg/with-sns.html (canonical SNS-to-Lambda event)
- **Captured on**: N/A (from docs)
- **Anonymization**: account ID in TopicArn, MessageId
- **Test it drives**: envelope/envelope_test.go::TestEnvelope/single_record
