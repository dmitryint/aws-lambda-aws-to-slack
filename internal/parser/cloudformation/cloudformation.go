// Package cloudformation renders Slack messages for AWS CloudFormation SNS
// stack-event notifications. Matches when the SNS Subject begins with
// "AWS CloudFormation Notification".
package cloudformation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/console"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

const (
	name          = "cloudformation"
	subjectPrefix = "AWS CloudFormation Notification"
	keyStackID    = "StackId"
	keyStackName  = "StackName"
	keyLogicalID  = "LogicalResourceId"
	keyStatus     = "ResourceStatus"
)

// statusMapping carries the rendered title and color for one CloudFormation
// stack status. Kept private so consumers go through statusMappings.
type statusMapping struct {
	title string
	color string
}

// statusMappings is the literal status → (title, color) table. Any status
// not in this table renders with an empty title and an empty color, which
// Slack then drops.
var statusMappings = map[string]statusMapping{
	"CREATE_COMPLETE":                              {"Stack creation complete", slack.ColorOK},
	"CREATE_IN_PROGRESS":                           {"Stack creation in progress", slack.ColorAccent},
	"CREATE_FAILED":                                {"Stack creation failed", slack.ColorCritical},
	"DELETE_COMPLETE":                              {"Stack deletion complete", slack.ColorOK},
	"DELETE_FAILED":                                {"Stack deletion failed", slack.ColorCritical},
	"DELETE_IN_PROGRESS":                           {"Stack deletion in progress", slack.ColorAccent},
	"REVIEW_IN_PROGRESS":                           {"Stack review in progress", slack.ColorAccent},
	"ROLLBACK_COMPLETE":                            {"Stack rollback complete", slack.ColorWarning},
	"ROLLBACK_FAILED":                              {"Stack rollback failed", slack.ColorCritical},
	"ROLLBACK_IN_PROGRESS":                         {"Stack rollback in progress", slack.ColorWarning},
	"UPDATE_COMPLETE":                              {"Stack update complete", slack.ColorOK},
	"UPDATE_COMPLETE_CLEANUP_IN_PROGRESS":          {"Stack update complete, cleanup in progress", slack.ColorAccent},
	"UPDATE_IN_PROGRESS":                           {"Stack update in progress", slack.ColorAccent},
	"UPDATE_ROLLBACK_COMPLETE":                     {"Stack update rollback complete", slack.ColorWarning},
	"UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS": {"Stack update rollback complete, cleanup in progress", slack.ColorWarning},
	"UPDATE_ROLLBACK_FAILED":                       {"Stack update rollback failed", slack.ColorCritical},
	"UPDATE_ROLLBACK_IN_PROGRESS":                  {"Stack update rollback in progress", slack.ColorWarning},
}

// Parser handles CloudFormation stack-event SNS notifications.
type Parser struct{}

// New returns a Parser ready to register with the router.
func New() *Parser { return &Parser{} }

// Name returns the stable parser identifier.
func (Parser) Name() string { return name }

// Match returns true when the SNS Subject starts with the CloudFormation
// prefix. The check is purely structural — payload parsing happens later
// in Parse.
func (Parser) Match(e *envelope.Event) bool {
	return strings.HasPrefix(e.Subject(), subjectPrefix)
}

// Parse renders the Slack message for a CloudFormation stack event. Returns
// (nil, nil) for two silencing paths:
//   - body does not parse into key/value pairs (no LogicalResourceId or
//     StackName).
//   - resource event for a non-stack member (LogicalResourceId != StackName).
func (Parser) Parse(_ context.Context, e *envelope.Event) (*slack.Message, error) {
	body := stringMessage(e.Message())
	fields := parseQuotedKeyValueLines(body)
	if _, ok := fields[keyLogicalID]; !ok {
		return nil, nil //nolint:nilnil // silence — body unparseable
	}
	if _, ok := fields[keyStackName]; !ok {
		return nil, nil //nolint:nilnil // silence — body unparseable
	}

	logicalID := fields[keyLogicalID]
	stackName := fields[keyStackName]
	if logicalID != stackName {
		return nil, nil //nolint:nilnil // silence — resource-level event
	}

	status := fields[keyStatus]
	stackID := fields[keyStackID]
	mapping := statusMappings[status]

	region := regionFromARN(stackID)
	if region == "" {
		region = e.Region()
	}

	encodedStackID := url.QueryEscape(stackID)
	consoleURL := console.URLWithFragment(
		region,
		"cloudformation/home",
		"stacks/"+encodedStackID+"/events",
	)

	title := slack.Link(consoleURL, mapping.title)
	author := "AWS CloudFormation"

	blocks := []slack.Block{
		slack.SectionBlock(fmt.Sprintf("*%s*\n_%s_", title, author)),
		slack.FieldsSection([]slack.TextObject{
			{Type: slack.TextTypeMrkdwn, Text: "*Stack Name*\n" + stackName},
			{Type: slack.TextTypeMrkdwn, Text: "*Status*\n" + status},
		}),
	}

	fallback := fmt.Sprintf("%s: %s", stackName, mapping.title)
	return slack.NewMessage(mapping.color, fallback, blocks...), nil
}

// stringMessage extracts the inner SNS message as a plain string.
func stringMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

// parseQuotedKeyValueLines reads CloudFormation's `Key='Value'` line
// format. The surrounding single quotes are trimmed from both ends.
func parseQuotedKeyValueLines(body string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(body, "\n") {
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.Trim(line[idx+1:], "'")
		out[key] = value
	}
	return out
}

// regionFromARN extracts the region segment of a CloudFormation stack ARN.
// Example: arn:aws:cloudformation:{region}:{account}:stack/{name}/{uuid}.
func regionFromARN(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}
