#!/usr/bin/env bash
#
# check_samples.sh — Cross-check the samples/ directory: every event-source
# fixture listed in the required array must exist, and every JSON file in
# the tree must parse as valid JSON.
#
# Fails (non-zero exit) if:
#   - any required fixture is missing,
#   - any *.json under samples/ is not valid JSON.
#
set -euo pipefail

# Resolve repo root (parent of scripts/).
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

required=(
  "samples/autoscaling/instance_launch_error.json"
  "samples/autoscaling/instance_launch.json"
  "samples/autoscaling/instance_terminate_error.json"
  "samples/autoscaling/instance_terminate.json"
  "samples/autoscaling/test_notification.json"
  "samples/awshealth/abuse_event.json"
  "samples/awshealth/account_notification.json"
  "samples/awshealth/issue_open_no_end_time.json"
  "samples/awshealth/issue_resolved_with_end_time.json"
  "samples/awshealth/multi_entity.json"
  "samples/awshealth/multi_language_description.json"
  "samples/awshealth/scheduled_change.json"
  "samples/batch/job_failed.json"
  "samples/batch/job_runnable.json"
  "samples/batch/job_running.json"
  "samples/batch/job_starting.json"
  "samples/batch/job_submitted.json"
  "samples/batch/job_succeeded.json"
  "samples/beanstalk/aborted_operation.json"
  "samples/beanstalk/deploy_failed.json"
  "samples/beanstalk/removed_instance.json"
  "samples/beanstalk/state_ok.json"
  "samples/beanstalk/state_red.json"
  "samples/beanstalk/state_yellow.json"
  "samples/cloudformation/malformed.json"
  "samples/cloudformation/resource_event_ignored.json"
  "samples/cloudformation/status_create_complete.json"
  "samples/cloudformation/status_create_failed.json"
  "samples/cloudformation/status_create_in_progress.json"
  "samples/cloudformation/status_delete_complete.json"
  "samples/cloudformation/status_delete_failed.json"
  "samples/cloudformation/status_delete_in_progress.json"
  "samples/cloudformation/status_review_in_progress.json"
  "samples/cloudformation/status_rollback_complete.json"
  "samples/cloudformation/status_rollback_failed.json"
  "samples/cloudformation/status_rollback_in_progress.json"
  "samples/cloudformation/status_update_complete_cleanup_in_progress.json"
  "samples/cloudformation/status_update_complete.json"
  "samples/cloudformation/status_update_in_progress.json"
  "samples/cloudformation/status_update_rollback_complete_cleanup_in_progress.json"
  "samples/cloudformation/status_update_rollback_complete.json"
  "samples/cloudformation/status_update_rollback_failed.json"
  "samples/cloudformation/status_update_rollback_in_progress.json"
  "samples/cloudwatch/alarm_china.json"
  "samples/cloudwatch/alarm_composite.json"
  "samples/cloudwatch/alarm_critical_single_metric.json"
  "samples/cloudwatch/alarm_govcloud.json"
  "samples/cloudwatch/alarm_lambda_with_log_link.json"
  "samples/cloudwatch/alarm_metric_math_alb_5xx.json"
  "samples/cloudwatch/alarm_metric_math.json"
  "samples/cloudwatch/alarm_ok.json"
  "samples/cloudwatch/alarm_warning_insufficient_data.json"
  "samples/codebuild/build_failed.json"
  "samples/codebuild/build_in_progress.json"
  "samples/codebuild/build_phase_change.json"
  "samples/codebuild/build_stopped.json"
  "samples/codebuild/build_succeeded.json"
  "samples/codecommit/pullrequest/pr_comment.json"
  "samples/codecommit/pullrequest/pr_created.json"
  "samples/codecommit/pullrequest/pr_merged.json"
  "samples/codecommit/pullrequest/pr_source_updated.json"
  "samples/codecommit/pullrequest/pr_status_closed_unmerged.json"
  "samples/codecommit/repository/ref_created_branch.json"
  "samples/codecommit/repository/ref_created_tag.json"
  "samples/codecommit/repository/ref_deleted_branch.json"
  "samples/codecommit/repository/ref_deleted_tag.json"
  "samples/codecommit/repository/ref_updated_branch.json"
  "samples/codedeploy/eventbridge/failure.json"
  "samples/codedeploy/eventbridge/ready.json"
  "samples/codedeploy/eventbridge/start.json"
  "samples/codedeploy/eventbridge/stop.json"
  "samples/codedeploy/eventbridge/success.json"
  "samples/codedeploy/sns/created.json"
  "samples/codedeploy/sns/failed.json"
  "samples/codedeploy/sns/ready.json"
  "samples/codedeploy/sns/stopped.json"
  "samples/codedeploy/sns/succeeded.json"
  "samples/codepipeline/action_failed.json"
  "samples/codepipeline/action_started.json"
  "samples/codepipeline/action_succeeded.json"
  "samples/codepipeline/approval/approval_days_to_expiry.json"
  "samples/codepipeline/approval/approval_hours_to_expiry.json"
  "samples/codepipeline/approval/approval_minutes_to_expiry.json"
  "samples/codepipeline/approval/approval_past_expiry.json"
  "samples/codepipeline/approval/approval_with_custom_data.json"
  "samples/codepipeline/pipeline_canceled.json"
  "samples/codepipeline/pipeline_failed.json"
  "samples/codepipeline/pipeline_resumed.json"
  "samples/codepipeline/pipeline_started.json"
  "samples/codepipeline/pipeline_stopped.json"
  "samples/codepipeline/pipeline_stopping.json"
  "samples/codepipeline/pipeline_succeeded.json"
  "samples/codepipeline/pipeline_superseded.json"
  "samples/codepipeline/stage_failed.json"
  "samples/codepipeline/stage_started.json"
  "samples/codepipeline/stage_succeeded.json"
  "samples/ecs/container_instance_state_change.json"
  "samples/ecs/deployment_state_change.json"
  "samples/ecs/service_action_deployment_failed.json"
  "samples/ecs/service_action_steady_state.json"
  "samples/ecs/service_action_task_placement_failure.json"
  "samples/ecs/task_state_pending.json"
  "samples/ecs/task_state_running.json"
  "samples/ecs/task_state_stopped.json"
  "samples/generic/eventbridge_unknown_source.json"
  "samples/generic/sns_unknown_topic.json"
  "samples/guardduty/aws_api_call.json"
  "samples/guardduty/dns_request.json"
  "samples/guardduty/kubernetes_api_call.json"
  "samples/guardduty/network_connection.json"
  "samples/guardduty/port_probe.json"
  "samples/guardduty/rds_login_attempt.json"
  "samples/guardduty/resource_access_key.json"
  "samples/guardduty/resource_eks.json"
  "samples/guardduty/resource_instance.json"
  "samples/guardduty/resource_s3_bucket.json"
  "samples/guardduty/severity_high.json"
  "samples/guardduty/severity_low.json"
  "samples/guardduty/severity_medium.json"
  "samples/inspector/classic/assessment_run_completed.json"
  "samples/inspector/classic/assessment_run_started.json"
  "samples/inspector/classic/assessment_run_state_changed.json"
  "samples/inspector/classic/enable_assessment_notifications.json"
  "samples/inspector/classic/finding_reported.json"
  "samples/inspector2/finding_critical_ecr.json"
  "samples/inspector2/finding_high_ec2.json"
  "samples/inspector2/finding_high_lambda.json"
  "samples/inspector2/finding_medium_silenced.json"
  "samples/rds/availability.json"
  "samples/rds/backup.json"
  "samples/rds/configuration_change.json"
  "samples/rds/creation.json"
  "samples/rds/deletion.json"
  "samples/rds/failover.json"
  "samples/rds/failure.json"
  "samples/rds/low_storage.json"
  "samples/rds/maintenance.json"
  "samples/rds/notification.json"
  "samples/rds/read_replica.json"
  "samples/rds/recovery.json"
  "samples/rds/restoration.json"
  "samples/rds/security_patching.json"
  "samples/ses/bounce/permanent_general.json"
  "samples/ses/bounce/permanent_no_email.json"
  "samples/ses/bounce/permanent_on_account_suppression_list.json"
  "samples/ses/bounce/permanent_suppressed.json"
  "samples/ses/bounce/transient_attachment_rejected.json"
  "samples/ses/bounce/transient_content_rejected.json"
  "samples/ses/bounce/transient_general.json"
  "samples/ses/bounce/transient_mailbox_full.json"
  "samples/ses/bounce/transient_message_too_large.json"
  "samples/ses/bounce/undetermined.json"
  "samples/ses/complaint/abuse.json"
  "samples/ses/complaint/auth_failure.json"
  "samples/ses/complaint/fraud.json"
  "samples/ses/complaint/not_spam.json"
  "samples/ses/complaint/other.json"
  "samples/ses/complaint/virus.json"
  "samples/ses/received/multi_recipient.json"
  "samples/ses/received/simple.json"
  "samples/ses/received/with_attachments.json"
  "samples/sns_envelope/multi_record.json"
  "samples/sns_envelope/record_with_json_string.json"
  "samples/sns_envelope/record_with_plain_text.json"
  "samples/sns_envelope/single_record.json"
)

missing=()
for f in "${required[@]}"; do
  if [[ ! -f "${f}" ]]; then
    missing+=("${f}")
  fi
done

if (( ${#missing[@]} > 0 )); then
  printf 'missing: %s\n' "${missing[@]}" >&2
  exit 1
fi

# JSON validity check across every fixture we actually have on disk.
invalid=()
while IFS= read -r f; do
  if ! jq -e . "${f}" > /dev/null 2>&1; then
    invalid+=("${f}")
  fi
done < <(find samples -type f -name '*.json')

if (( ${#invalid[@]} > 0 )); then
  printf 'invalid JSON: %s\n' "${invalid[@]}" >&2
  exit 1
fi

count=$(find samples -type f -name '*.json' | wc -l | tr -d ' ')
echo "ok: ${count} sample fixtures, all valid JSON, all required files present"
