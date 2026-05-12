package codedeploy

import (
	"fmt"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/console"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

// renderInput captures the per-event fields the shared Slack-message builder
// reads. The SNS and EventBridge parsers populate it from their respective
// payload shapes, then call renderMessage to produce the final attachment.
type renderInput struct {
	region          string
	application     string
	deploymentID    string
	deploymentGroup string
	status          string
	titleSuffix     string
	color           string
}

// renderMessage builds the CodeDeploy Slack message shared by the SNS and
// EventBridge variants. The two callers differ only in the status keyword and
// the matching title suffix (e.g. "SUCCEEDED" → " has finished" for SNS,
// "SUCCESS" → " has finished" for EventBridge); everything else is identical
// and lives here.
func renderMessage(in renderInput) *slack.Message {
	consoleURL := console.URLWithFragment(in.region, "codedeploy/home", "/deployments/"+in.deploymentID)
	baseTitle := slack.Link(consoleURL, "CodeDeploy Application "+in.application)

	title := baseTitle + in.titleSuffix
	color := in.color
	if color == "" {
		color = slack.ColorNeutral
	}

	author := "AWS CodeDeploy Notification"
	fieldsObjs := make([]slack.TextObject, 0, 2)
	if in.status != "" {
		fieldsObjs = append(fieldsObjs, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*Status*\n" + in.status,
		})
	}
	fieldsObjs = append(fieldsObjs, slack.TextObject{
		Type: slack.TextTypeMrkdwn,
		Text: "*DeploymentGroup*\n" + in.deploymentGroup,
	})

	blocks := []slack.Block{
		slack.SectionBlock(fmt.Sprintf("*%s*\n_%s_", title, author)),
		slack.FieldsSection(fieldsObjs),
	}

	fallback := fmt.Sprintf("%s %s", baseTitle, in.status)
	return slack.NewMessage(color, fallback, blocks...)
}

// statusOutcome is the (titleSuffix, color) pair the variant-specific status
// keyword maps to. Both parsers translate their native status strings into
// these outcomes before calling renderMessage.
type statusOutcome struct {
	titleSuffix string
	color       string
}

// neutralOutcome represents the unknown / in-progress status — no title
// suffix, neutral color.
var neutralOutcome = statusOutcome{color: slack.ColorNeutral}

// outcomeFinished marks a successful deployment.
var outcomeFinished = statusOutcome{titleSuffix: " has finished", color: slack.ColorOK}

// outcomeStopped marks a deployment the user halted.
var outcomeStopped = statusOutcome{titleSuffix: " was stopped", color: slack.ColorWarning}

// outcomeFailed marks a deployment that failed.
var outcomeFailed = statusOutcome{titleSuffix: " has failed", color: slack.ColorCritical}

// outcomeStarted marks a deployment that just kicked off.
var outcomeStarted = statusOutcome{titleSuffix: " has started deploying", color: slack.ColorNeutral}
