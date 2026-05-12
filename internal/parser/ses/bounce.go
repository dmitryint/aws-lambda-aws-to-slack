package ses

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

const (
	bounceName = "ses-bounce"

	bounceTypeTransient = "Transient"
	bounceTypePermanent = "Permanent"
)

// BounceParser handles SES bounce notifications. Matches when the inner
// SNS message carries notificationType "Bounce".
type BounceParser struct{}

// NewBounce returns a BounceParser ready to register with the router.
func NewBounce() *BounceParser { return &BounceParser{} }

// Name returns the stable parser identifier.
func (BounceParser) Name() string { return bounceName }

// Match returns true when the inner SNS message has notificationType "Bounce".
func (BounceParser) Match(e *envelope.Event) bool {
	return matchNotification(e, notifBounce)
}

// bouncePayload is the subset of the SES bounce SNS payload the parser
// reads.
type bouncePayload struct {
	Bounce bounceDetail `json:"bounce"`
	Mail   commonMail   `json:"mail"`
}

// bounceDetail captures the bounce.* block.
type bounceDetail struct {
	BounceType        string             `json:"bounceType"`
	BounceSubType     string             `json:"bounceSubType"`
	BouncedRecipients []bouncedRecipient `json:"bouncedRecipients"`
}

// bouncedRecipient is one element of bounce.bouncedRecipients.
type bouncedRecipient struct {
	EmailAddress   string `json:"emailAddress"`
	Action         string `json:"action"`
	Status         string `json:"status"`
	DiagnosticCode string `json:"diagnosticCode"`
}

// Parse renders the Slack message for an SES Bounce notification.
func (BounceParser) Parse(_ context.Context, e *envelope.Event) (*slack.Message, error) {
	p, ok := decodeBounce(e)
	if !ok {
		return nil, fmt.Errorf("ses-bounce: payload missing or not an object")
	}

	color := bounceColor(p.Bounce.BounceType)
	author := fmt.Sprintf("Amazon SES - Bounce: %s - %s",
		p.Bounce.BounceType, p.Bounce.BounceSubType)
	title := p.Mail.CommonHeaders.Subject

	fields := buildMailFields(p.Mail)
	fields = append(fields,
		slack.TextObject{Type: slack.TextTypeMrkdwn, Text: "*BounceType*\n" + p.Bounce.BounceType},
		slack.TextObject{Type: slack.TextTypeMrkdwn, Text: "*BounceSubType*\n" + p.Bounce.BounceSubType},
	)

	blocks := []slack.Block{
		slack.SectionBlock(fmt.Sprintf("*%s*\n_%s_", title, author)),
	}
	if body := renderBouncedRecipients(p.Bounce.BouncedRecipients); body != "" {
		blocks = append(blocks, slack.SectionBlock(body))
	}
	blocks = append(blocks, slack.FieldsSection(fields))

	fallback := fmt.Sprintf("Bounce: %s - %s",
		p.Bounce.BounceType, p.Bounce.BounceSubType)
	return slack.NewMessage(color, fallback, blocks...), nil
}

// decodeBounce decodes the typed bounce payload from the inner SNS message.
func decodeBounce(e *envelope.Event) (bouncePayload, bool) {
	raw := e.Message()
	if len(raw) == 0 || raw[0] != '{' {
		return bouncePayload{}, false
	}
	var p bouncePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return bouncePayload{}, false
	}
	return p, true
}

// bounceColor maps the bounceType to a Slack color: Transient → accent,
// Permanent → critical, anything else → neutral.
func bounceColor(bounceType string) string {
	switch bounceType {
	case bounceTypeTransient:
		return slack.ColorAccent
	case bounceTypePermanent:
		return slack.ColorCritical
	default:
		return slack.ColorNeutral
	}
}

// buildMailFields constructs the shared From / To rows the SES bounce and
// complaint parsers both emit. The Received parser uses a different format
// so does not call this helper.
func buildMailFields(m commonMail) []slack.TextObject {
	fields := make([]slack.TextObject, 0, 4)
	if m.Source != "" {
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*From*\n" + m.Source,
		})
	}
	if len(m.Destination) > 0 {
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*To*\n" + strings.Join(m.Destination, ",\n"),
		})
	}
	return fields
}

// renderBouncedRecipients lists one human-readable line per bouncedRecipient
// — email plus action/status/diagnosticCode when present. An empty slice
// yields an empty string so the caller can skip the body section.
func renderBouncedRecipients(recipients []bouncedRecipient) string {
	if len(recipients) == 0 {
		return ""
	}
	lines := make([]string, 0, len(recipients))
	for _, r := range recipients {
		var details []string
		if r.Action != "" {
			details = append(details, r.Action)
		}
		if r.Status != "" {
			details = append(details, r.Status)
		}
		if r.DiagnosticCode != "" {
			details = append(details, r.DiagnosticCode)
		}
		if len(details) == 0 {
			lines = append(lines, r.EmailAddress)
			continue
		}
		lines = append(lines, fmt.Sprintf("%s — %s", r.EmailAddress, strings.Join(details, "; ")))
	}
	return strings.Join(lines, "\n")
}
