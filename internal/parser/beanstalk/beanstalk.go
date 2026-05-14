// Package beanstalk renders AWS Elastic Beanstalk SNS notifications into the
// transport-neutral notify.Notification shape. Matches when the SNS Subject
// begins with "AWS Elastic Beanstalk Notification".
package beanstalk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

const (
	name           = "beanstalk"
	subjectPrefix  = "AWS Elastic Beanstalk Notification"
	keyMessage     = "Message"
	keyApplication = "Application"
	keyEnvironment = "Environment"
	keyTimestamp   = "Timestamp"
	keyEnvURL      = "Environment URL"

	authorBeanstalk = "AWS Elastic Beanstalk"
)

// Parser handles Beanstalk SNS notifications.
type Parser struct{}

// New returns a Parser ready to register with the router.
func New() *Parser { return &Parser{} }

// Name returns the stable parser identifier.
func (Parser) Name() string { return name }

// Match returns true when the SNS Subject starts with the Beanstalk prefix.
func (Parser) Match(e *envelope.Event) bool {
	return strings.HasPrefix(e.Subject(), subjectPrefix)
}

// Parse renders the Notification for a Beanstalk SNS notification. Returns
// (nil, nil) when the body is not parseable into the required key/value
// pairs.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*notify.Notification, error) {
	body := stringMessage(e.Message())
	fields := parseKeyValueLines(body)
	if !hasRequiredFields(fields) {
		return nil, nil //nolint:nilnil // silence — required fields missing
	}

	text := fields[keyMessage]
	application := fields[keyApplication]
	environment := fields[keyEnvironment]
	envURL := fields[keyEnvURL]

	severity := classify(text)
	titleText := fmt.Sprintf("%s / %s", application, environment)

	return &notify.Notification{
		Source:   name,
		Severity: severity,
		Title:    titleText,
		TitleURL: envURL,
		Subtitle: authorBeanstalk,
		Summary:  text,
		Fields: []notify.Field{
			{Key: "Application", Value: application},
			{Key: "Environment", Value: environment},
		},
		Fallback: fmt.Sprintf("%s / %s: %s", application, environment, text),
	}, nil
}

// stringMessage extracts the inner SNS message as a plain string. Beanstalk
// uses a free-form key:value payload, so the envelope wraps it as a JSON
// string we unwrap here.
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

// parseKeyValueLines splits the Beanstalk SNS body on newlines and extracts
// "Key: Value" pairs.
func parseKeyValueLines(body string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(body, "\n") {
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		out[key] = value
	}
	return out
}

// hasRequiredFields returns true when the four Beanstalk fields the parser
// renders are all present.
func hasRequiredFields(fields map[string]string) bool {
	for _, k := range []string{keyMessage, keyApplication, keyEnvironment, keyTimestamp} {
		if _, ok := fields[k]; !ok {
			return false
		}
	}
	return true
}

// classify maps the Beanstalk Message text to a Severity. When both the
// critical and warning substring lists match the same message, warning
// wins because its loop runs after the critical loop.
func classify(text string) notify.Severity {
	critical := []string{
		" to RED",
		" to Severe",
		" but with errors",
		"You do not have permission",
		"Failed to deploy application",
		"Failed to deploy configuration",
		"Your quota allows for 0 more running instance",
		"Unsuccessful command execution",
	}
	warning := []string{
		" to YELLOW",
		" to Warning",
		" to Degraded",
		" to Info",
		"Removed instance ",
		"Adding instance ",
		" aborted operation.",
		"some instances may have deployed the new application version",
	}
	severity := notify.SeverityInfo
	for _, substr := range critical {
		if strings.Contains(text, substr) {
			severity = notify.SeverityCritical
			break
		}
	}
	for _, substr := range warning {
		if strings.Contains(text, substr) {
			severity = notify.SeverityWarning
			break
		}
	}
	return severity
}
