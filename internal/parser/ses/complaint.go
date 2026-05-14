package ses

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

const (
	complaintName = "ses-complaint"

	complaintAbuse = "abuse"
	complaintVirus = "virus"
)

// ComplaintParser handles SES complaint notifications. Matches when the
// inner SNS message carries notificationType "Complaint".
type ComplaintParser struct{}

// NewComplaint returns a ComplaintParser ready to register with the router.
func NewComplaint() *ComplaintParser { return &ComplaintParser{} }

// Name returns the stable parser identifier.
func (ComplaintParser) Name() string { return complaintName }

// Match returns true when the inner SNS message has notificationType
// "Complaint".
func (ComplaintParser) Match(e *envelope.Event) bool {
	return matchNotification(e, notifComplaint)
}

// complaintPayload is the subset of the SES complaint SNS payload the
// parser reads.
type complaintPayload struct {
	Complaint complaintDetail `json:"complaint"`
	Mail      commonMail      `json:"mail"`
}

// complaintDetail captures the complaint.* block.
type complaintDetail struct {
	UserAgent             string                `json:"userAgent"`
	ComplaintFeedbackType string                `json:"complaintFeedbackType"`
	ComplainedRecipients  []complainedRecipient `json:"complainedRecipients"`
}

// complainedRecipient is one element of complaint.complainedRecipients.
type complainedRecipient struct {
	EmailAddress string `json:"emailAddress"`
}

// Parse renders the Notification for an SES Complaint notification.
func (ComplaintParser) Parse(_ context.Context, e *envelope.Event) (*notify.Notification, error) {
	p, ok := decodeComplaint(e)
	if !ok {
		return nil, fmt.Errorf("ses-complaint: payload missing or not an object")
	}

	severity := complaintSeverity(p.Complaint.ComplaintFeedbackType)
	subtitle := fmt.Sprintf("Amazon SES - Complaint: %s", p.Complaint.UserAgent)
	title := p.Mail.CommonHeaders.Subject

	fields := buildMailFields(p.Mail)
	fields = append(fields,
		notify.Field{Key: "UserAgent", Value: p.Complaint.UserAgent},
		notify.Field{Key: "Complain Type", Value: p.Complaint.ComplaintFeedbackType},
	)

	summary := renderComplainedRecipients(p.Complaint.ComplainedRecipients)
	fallback := fmt.Sprintf("Complaint: %s", p.Complaint.UserAgent)

	return &notify.Notification{
		Source:   complaintName,
		Severity: severity,
		Title:    title,
		Subtitle: subtitle,
		Summary:  summary,
		Fields:   fields,
		Fallback: fallback,
	}, nil
}

// decodeComplaint decodes the typed complaint payload from the inner SNS
// message.
func decodeComplaint(e *envelope.Event) (complaintPayload, bool) {
	raw := e.Message()
	if len(raw) == 0 || raw[0] != '{' {
		return complaintPayload{}, false
	}
	var p complaintPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return complaintPayload{}, false
	}
	return p, true
}

// complaintSeverity maps the complaintFeedbackType to a Severity. RFC 6650
// "abuse" / "virus" are Critical per the spec; any other complaint type is
// Warning.
func complaintSeverity(feedbackType string) notify.Severity {
	switch strings.ToLower(feedbackType) {
	case complaintAbuse, complaintVirus:
		return notify.SeverityCritical
	default:
		return notify.SeverityWarning
	}
}

// renderComplainedRecipients lists one email per line. An empty slice
// yields an empty string so the caller can omit the body section.
func renderComplainedRecipients(recipients []complainedRecipient) string {
	if len(recipients) == 0 {
		return ""
	}
	lines := make([]string, 0, len(recipients))
	for _, r := range recipients {
		lines = append(lines, r.EmailAddress)
	}
	return strings.Join(lines, "\n")
}
