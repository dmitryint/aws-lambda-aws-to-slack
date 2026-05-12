// Package beanstalk renders Slack messages for AWS Elastic Beanstalk SNS
// notifications. Matches when the SNS Subject begins with "AWS Elastic
// Beanstalk Notification".
package beanstalk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

const (
	name           = "beanstalk"
	subjectPrefix  = "AWS Elastic Beanstalk Notification"
	keyMessage     = "Message"
	keyApplication = "Application"
	keyEnvironment = "Environment"
	keyTimestamp   = "Timestamp"
	keyEnvURL      = "Environment URL"
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

// Parse renders the Slack message for a Beanstalk notification. Returns
// (nil, nil) when the body is not parseable into the required key/value
// pairs.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*slack.Message, error) {
	body := stringMessage(e.Message())
	fields := parseKeyValueLines(body)
	if !hasRequiredFields(fields) {
		return nil, nil //nolint:nilnil // silence — required fields missing
	}

	text := fields[keyMessage]
	application := fields[keyApplication]
	environment := fields[keyEnvironment]
	envURL := fields[keyEnvURL]

	color := classify(text)
	titleText := fmt.Sprintf("%s / %s", application, environment)
	title := slack.Link(envURL, titleText)
	author := "AWS Elastic Beanstalk"

	blocks := []slack.Block{
		slack.SectionBlock(fmt.Sprintf("*%s*\n_%s_", title, author)),
		slack.SectionBlock(text),
		slack.FieldsSection([]slack.TextObject{
			{Type: slack.TextTypeMrkdwn, Text: "*Application*\n" + application},
			{Type: slack.TextTypeMrkdwn, Text: "*Environment*\n" + environment},
		}),
	}

	fallback := fmt.Sprintf("%s / %s: %s", application, environment, text)
	return slack.NewMessage(color, fallback, blocks...), nil
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

// classify maps the Beanstalk Message text to a Slack color. When both the
// critical and warning substring lists match the same message, warning
// wins because its loop runs after the critical loop.
func classify(text string) string {
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
	color := slack.ColorOK
	for _, substr := range critical {
		if strings.Contains(text, substr) {
			color = slack.ColorCritical
			break
		}
	}
	for _, substr := range warning {
		if strings.Contains(text, substr) {
			color = slack.ColorWarning
			break
		}
	}
	return color
}
