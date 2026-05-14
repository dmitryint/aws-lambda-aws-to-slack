package codedeploy

import (
	"fmt"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/console"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

const (
	authorCodeDeploy = "AWS CodeDeploy Notification"
)

// renderInput captures the per-event fields the shared Notification builder
// reads. The SNS and EventBridge parsers populate it from their respective
// payload shapes, then call renderNotification to produce the final value.
type renderInput struct {
	source          string
	region          string
	application     string
	deploymentID    string
	deploymentGroup string
	status          string
	titleSuffix     string
	severity        notify.Severity
}

// renderNotification builds the CodeDeploy Notification shared by the SNS
// and EventBridge variants. The two callers differ only in the status
// keyword and the matching title suffix (e.g. "SUCCEEDED" → " has finished"
// for SNS, "SUCCESS" → " has finished" for EventBridge); everything else is
// identical and lives here.
func renderNotification(in renderInput) *notify.Notification {
	consoleURL := console.URLWithFragment(in.region, "codedeploy/home", "/deployments/"+in.deploymentID)
	baseTitle := "CodeDeploy Application " + in.application

	severity := in.severity
	if severity == notify.SeverityUnknown {
		severity = notify.SeverityNotice
	}

	fields := make([]notify.Field, 0, 2)
	if in.status != "" {
		fields = append(fields, notify.Field{Key: "Status", Value: in.status})
	}
	fields = append(fields, notify.Field{Key: "DeploymentGroup", Value: in.deploymentGroup})

	return &notify.Notification{
		Source:   in.source,
		Severity: severity,
		Title:    baseTitle + in.titleSuffix,
		TitleURL: consoleURL,
		Subtitle: authorCodeDeploy,
		Fields:   fields,
		Fallback: fmt.Sprintf("%s %s", baseTitle, in.status),
	}
}

// statusOutcome is the (titleSuffix, severity) pair the variant-specific
// status keyword maps to. Both parsers translate their native status strings
// into these outcomes before calling renderNotification.
type statusOutcome struct {
	titleSuffix string
	severity    notify.Severity
}

// neutralOutcome represents the unknown / in-progress status — no title
// suffix, Notice severity (default per the spec).
var neutralOutcome = statusOutcome{severity: notify.SeverityNotice}

// outcomeFinished marks a successful deployment.
var outcomeFinished = statusOutcome{titleSuffix: " has finished", severity: notify.SeverityOK}

// outcomeStopped marks a deployment the user halted.
var outcomeStopped = statusOutcome{titleSuffix: " was stopped", severity: notify.SeverityNotice}

// outcomeFailed marks a deployment that failed.
var outcomeFailed = statusOutcome{titleSuffix: " has failed", severity: notify.SeverityCritical}

// outcomeStarted marks a deployment that just kicked off.
var outcomeStarted = statusOutcome{titleSuffix: " has started deploying", severity: notify.SeverityNotice}
