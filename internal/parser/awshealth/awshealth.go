// Package awshealth renders Slack messages for AWS Health Dashboard
// EventBridge events. Matches when the EventBridge source is "aws.health".
//
// Two detail-types reach this parser: "AWS Health Event" (regional service
// status notifications, account notifications, scheduled changes) and "AWS
// Health Abuse Event" (account-wide abuse reports). Both share the same
// detail shape but differ in the eventTypeCategory; they flow through the
// same render path.
package awshealth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

const (
	name         = "awshealth"
	sourceHealth = "health"

	categoryIssue   = "issue"
	preferredLang   = "en_US"
	timeRenderForm  = "Mon, 02 Jan 2006 15:04:05 MST"
	descriptionLen2 = 3
)

// Parser handles AWS Health Dashboard EventBridge events.
type Parser struct{}

// New returns a Parser ready to register with the router.
func New() *Parser { return &Parser{} }

// Name returns the stable parser identifier.
func (Parser) Name() string { return name }

// Match returns true when the EventBridge source identifies an AWS Health
// event. Both "AWS Health Event" and "AWS Health Abuse Event" carry source
// "aws.health" and flow through the same parser body.
func (Parser) Match(e *envelope.Event) bool {
	return e.Source() == sourceHealth
}

// detail captures the fields the parser reads from detail.*.
type detail struct {
	Service           string             `json:"service"`
	EventTypeCategory string             `json:"eventTypeCategory"`
	EventDescription  []descriptionEntry `json:"eventDescription"`
	AffectedEntities  []affectedEntity   `json:"affectedEntities"`
	StartTime         string             `json:"startTime"`
	EndTime           string             `json:"endTime"`
}

// descriptionEntry is one locale-specific description block.
type descriptionEntry struct {
	Language          string `json:"language"`
	LatestDescription string `json:"latestDescription"`
}

// affectedEntity is one element of the affectedEntities array.
type affectedEntity struct {
	EntityValue string `json:"entityValue"`
}

// Parse renders the Slack message for an AWS Health event.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*slack.Message, error) {
	d, ok := decodeDetail(e)
	if !ok {
		return nil, fmt.Errorf("awshealth: detail block missing or malformed")
	}

	description := pickDescription(d.EventDescription)
	color := slack.ColorAccent
	if d.EventTypeCategory == categoryIssue {
		color = slack.ColorWarning
	}

	detailType := e.DetailType()
	accountID := e.AccountID()

	fields := buildFields(accountID, d)

	blocks := []slack.Block{
		slack.SectionBlock(fmt.Sprintf("*%s*\n%s", detailType, formatMrkdwn(description))),
	}
	if len(fields) > 0 {
		blocks = append(blocks, slack.FieldsSection(fields))
	}

	fallback := description
	if fallback == "" {
		fallback = detailType
	}
	return slack.NewMessage(color, fallback, blocks...), nil
}

// decodeDetail extracts the typed detail block from the inner event message.
func decodeDetail(e *envelope.Event) (detail, bool) {
	raw := e.Get("detail")
	if len(raw) == 0 {
		return detail{}, false
	}
	var d detail
	if err := json.Unmarshal(raw, &d); err != nil {
		return detail{}, false
	}
	return d, true
}

// pickDescription returns the en_US latestDescription when present,
// otherwise the first element's latestDescription.
func pickDescription(entries []descriptionEntry) string {
	for _, entry := range entries {
		if entry.Language == preferredLang {
			return entry.LatestDescription
		}
	}
	if len(entries) > 0 {
		return entries[0].LatestDescription
	}
	return ""
}

// buildFields constructs the field rows shown in the rendered alert. The
// Account ID field is always present; Service / Start Time / End Time /
// Affected Entities are conditional.
func buildFields(accountID string, d detail) []slack.TextObject {
	fields := make([]slack.TextObject, 0, 5)
	fields = append(fields, slack.TextObject{
		Type: slack.TextTypeMrkdwn,
		Text: "*Account ID*\n" + accountID,
	})
	if d.Service != "" {
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*Service*\n" + d.Service,
		})
	}
	if d.StartTime != "" {
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*Start Time*\n" + formatHealthTime(d.StartTime),
		})
	}
	if d.EndTime != "" {
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*End Time*\n" + formatHealthTime(d.EndTime),
		})
	}
	if len(d.AffectedEntities) > 0 {
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*Affected Entities*\n" + joinEntities(d.AffectedEntities),
		})
	}
	return fields
}

// joinEntities reproduces `_.join(_.map(affectedEntities, "entityValue"), "\n")`.
func joinEntities(entities []affectedEntity) string {
	values := make([]string, 0, len(entities))
	for _, ent := range entities {
		values = append(values, ent.EntityValue)
	}
	return strings.Join(values, "\n")
}

// formatHealthTime parses the AWS Health time string and renders it in a
// stable UTC form. AWS Health emits an RFC-1123 style timestamp (for
// example "Sat, 1 Jun 2024 11:30:00 GMT") in the start/endTime fields. A
// fixed layout keeps golden outputs reproducible. When the value cannot
// be parsed it is passed through verbatim.
func formatHealthTime(s string) string {
	for _, layout := range []string{
		time.RFC1123,
		"Mon, 2 Jan 2006 15:04:05 MST",
		"Mon, 02 Jan 2006 15:04:05 MST",
		time.RFC3339,
		time.RFC3339Nano,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(timeRenderForm)
		}
	}
	return s
}

// formatMrkdwn substitutes the literal sequence "//n" (case-insensitive)
// with a newline. AWS Health uses this escape to mark line breaks inside
// the eventDescription text.
func formatMrkdwn(text string) string {
	if text == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(text))
	i := 0
	for i < len(text) {
		if i+descriptionLen2 <= len(text) {
			seg := text[i : i+descriptionLen2]
			if seg == "//n" || seg == "//N" {
				b.WriteByte('\n')
				i += descriptionLen2
				continue
			}
		}
		b.WriteByte(text[i])
		i++
	}
	return b.String()
}
