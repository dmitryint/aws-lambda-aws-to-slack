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
	receivedName = "ses-received"

	mimeMultipart = "multipart/"
	attachmentHdr = "Content-Disposition: attachment"
)

// ReceivedParser handles SES inbound-receipt SNS notifications. Matches
// when the inner SNS message carries notificationType "Received".
type ReceivedParser struct{}

// NewReceived returns a ReceivedParser ready to register with the router.
func NewReceived() *ReceivedParser { return &ReceivedParser{} }

// Name returns the stable parser identifier.
func (ReceivedParser) Name() string { return receivedName }

// Match returns true when the inner SNS message has notificationType
// "Received".
func (ReceivedParser) Match(e *envelope.Event) bool {
	return matchNotification(e, notifReceived)
}

// receivedPayload is the subset of the SES received SNS payload the parser
// reads. The raw `content` field is consumed only as a heuristic input to
// hasAttachments and is never rendered into the Slack message.
type receivedPayload struct {
	Mail    receivedMail `json:"mail"`
	Content string       `json:"content"`
}

// receivedMail extends commonMail with the headers slice the parser walks
// to detect attachments.
type receivedMail struct {
	Source        string        `json:"source"`
	Timestamp     string        `json:"timestamp"`
	Destination   []string      `json:"destination"`
	CommonHeaders commonHeaders `json:"commonHeaders"`
	Headers       []mailHeader  `json:"headers"`
}

// mailHeader is one element of mail.headers.
type mailHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Parse renders the Slack message for an SES inbound-receipt notification.
//
// For privacy reasons the raw RFC 5322 message body is never sent to
// Slack — only the From / To / Subject metadata is rendered, plus a
// "has attachments" hint when the message is multipart.
func (ReceivedParser) Parse(_ context.Context, e *envelope.Event) (*slack.Message, error) {
	p, ok := decodeReceived(e)
	if !ok {
		return nil, fmt.Errorf("ses-received: payload missing or not an object")
	}

	subject := p.Mail.CommonHeaders.Subject
	author := "Amazon SES"

	fields := make([]slack.TextObject, 0, 3)
	if p.Mail.Source != "" {
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*From*\n" + p.Mail.Source,
		})
	}
	if len(p.Mail.Destination) > 0 {
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*To*\n" + strings.Join(p.Mail.Destination, ",\n"),
		})
	}
	if hasAttachments(p.Mail.Headers, p.Content) {
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*Attachments*\nMessage carries one or more attachments.",
		})
	}

	blocks := []slack.Block{
		slack.SectionBlock(fmt.Sprintf("*%s*\n_%s_", subject, author)),
	}
	if len(fields) > 0 {
		blocks = append(blocks, slack.FieldsSection(fields))
	}

	fallback := "New email received from SES"
	return slack.NewMessage(slack.ColorAccent, fallback, blocks...), nil
}

// decodeReceived decodes the typed received payload from the inner SNS
// message.
func decodeReceived(e *envelope.Event) (receivedPayload, bool) {
	raw := e.Message()
	if len(raw) == 0 || raw[0] != '{' {
		return receivedPayload{}, false
	}
	var p receivedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return receivedPayload{}, false
	}
	return p, true
}

// hasAttachments returns true when the message looks like a multipart with
// at least one attachment part. We check the parsed mail.headers for a
// Content-Type that starts with "multipart/" and, if available, look at the
// raw content for the "Content-Disposition: attachment" marker. The body
// scan is a coarse heuristic — sufficient to add the "has attachments"
// hint without parsing the full MIME tree.
func hasAttachments(headers []mailHeader, content string) bool {
	multipart := false
	for _, h := range headers {
		if strings.EqualFold(h.Name, "Content-Type") &&
			strings.HasPrefix(h.Value, mimeMultipart) {
			multipart = true
			break
		}
	}
	if !multipart {
		return false
	}
	return strings.Contains(content, attachmentHdr)
}
