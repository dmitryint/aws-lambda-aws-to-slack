// Package cloudformation renders AWS CloudFormation SNS stack-event
// notifications into the transport-neutral notify.Notification shape.
// Matches when the SNS Subject begins with "AWS CloudFormation Notification".
package cloudformation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/console"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

const (
	name          = "cloudformation"
	subjectPrefix = "AWS CloudFormation Notification"
	keyStackID    = "StackId"
	keyStackName  = "StackName"
	keyLogicalID  = "LogicalResourceId"
	keyStatus     = "ResourceStatus"

	authorCloudFormation = "AWS CloudFormation"
)

// statusMapping carries the rendered title and severity for one CloudFormation
// stack status. Kept private so consumers go through statusMappings.
type statusMapping struct {
	title    string
	severity notify.Severity
}

// statusMappings is the literal status → (title, severity) table.
// `*_FAILED` → Critical, `*_ROLLBACK_COMPLETE` → Warning, `*_COMPLETE` (success)
// → OK, `*_IN_PROGRESS` → Notice.
var statusMappings = map[string]statusMapping{
	"CREATE_COMPLETE":                              {"Stack creation complete", notify.SeverityOK},
	"CREATE_IN_PROGRESS":                           {"Stack creation in progress", notify.SeverityNotice},
	"CREATE_FAILED":                                {"Stack creation failed", notify.SeverityCritical},
	"DELETE_COMPLETE":                              {"Stack deletion complete", notify.SeverityOK},
	"DELETE_FAILED":                                {"Stack deletion failed", notify.SeverityCritical},
	"DELETE_IN_PROGRESS":                           {"Stack deletion in progress", notify.SeverityNotice},
	"REVIEW_IN_PROGRESS":                           {"Stack review in progress", notify.SeverityNotice},
	"ROLLBACK_COMPLETE":                            {"Stack rollback complete", notify.SeverityWarning},
	"ROLLBACK_FAILED":                              {"Stack rollback failed", notify.SeverityCritical},
	"ROLLBACK_IN_PROGRESS":                         {"Stack rollback in progress", notify.SeverityWarning},
	"UPDATE_COMPLETE":                              {"Stack update complete", notify.SeverityOK},
	"UPDATE_COMPLETE_CLEANUP_IN_PROGRESS":          {"Stack update complete, cleanup in progress", notify.SeverityNotice},
	"UPDATE_IN_PROGRESS":                           {"Stack update in progress", notify.SeverityNotice},
	"UPDATE_ROLLBACK_COMPLETE":                     {"Stack update rollback complete", notify.SeverityWarning},
	"UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS": {"Stack update rollback complete, cleanup in progress", notify.SeverityWarning},
	"UPDATE_ROLLBACK_FAILED":                       {"Stack update rollback failed", notify.SeverityCritical},
	"UPDATE_ROLLBACK_IN_PROGRESS":                  {"Stack update rollback in progress", notify.SeverityWarning},
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

// Parse renders the Notification for a CloudFormation stack event. Returns
// (nil, nil) for two silencing paths:
//   - body does not parse into key/value pairs (no LogicalResourceId or
//     StackName).
//   - resource event for a non-stack member (LogicalResourceId != StackName).
func (Parser) Parse(_ context.Context, e *envelope.Event) (*notify.Notification, error) {
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

	severity := mapping.severity
	if severity == notify.SeverityUnknown {
		severity = notify.SeverityNotice
	}

	return &notify.Notification{
		Source:   name,
		Severity: severity,
		Title:    mapping.title,
		TitleURL: consoleURL,
		Subtitle: authorCloudFormation,
		Fields: []notify.Field{
			{Key: "Stack Name", Value: stackName},
			{Key: "Status", Value: status},
		},
		Fallback: fmt.Sprintf("%s: %s", stackName, mapping.title),
	}, nil
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
