// Package ses renders Slack messages for Amazon SES notification SNS events.
//
// Three sibling parsers live in this package — one per notificationType the
// SES SNS feed emits:
//
//   - Bounce parser (bounce.go) — notificationType == "Bounce"
//   - Complaint parser (complaint.go) — notificationType == "Complaint"
//   - Received parser (received.go) — notificationType == "Received"
//
// All three discriminate on the same field of the inner SNS message, so the
// shared decode and Match helpers live in this file. The matcher is cheap
// (no allocation beyond the existing envelope buffer for typical inputs)
// and intentionally narrow — it reads only notificationType so a sibling
// parser cannot accidentally claim another's samples.
//
// There is no Email-out / sender sidecar — only the three notification
// parsers.
package ses

import (
	"encoding/json"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
)

// notificationType is the SES SNS payload discriminator. Each parser owns
// exactly one value: "Bounce", "Complaint", or "Received".
const (
	notifBounce    = "Bounce"
	notifComplaint = "Complaint"
	notifReceived  = "Received"
)

// commonMail captures the `mail` block shared across all three SES
// notification shapes — Bounce, Complaint, and Received all carry the same
// envelope metadata in this block, so the three parsers decode it through
// the same struct.
type commonMail struct {
	Source        string        `json:"source"`
	Timestamp     string        `json:"timestamp"`
	Destination   []string      `json:"destination"`
	CommonHeaders commonHeaders `json:"commonHeaders"`
}

// commonHeaders captures the subset of mail.commonHeaders the parsers read.
type commonHeaders struct {
	Subject string `json:"subject"`
}

// matchNotification returns true when the envelope's inner SNS message has a
// JSON object payload whose `notificationType` field equals the given value.
// Bounce / Complaint / Received parsers all call this with their own
// discriminator constant, keeping the matchers strict and disjoint.
func matchNotification(e *envelope.Event, want string) bool {
	raw := e.Message()
	if len(raw) == 0 || raw[0] != '{' {
		return false
	}
	var hdr struct {
		NotificationType string `json:"notificationType"`
	}
	if err := json.Unmarshal(raw, &hdr); err != nil {
		return false
	}
	return hdr.NotificationType == want
}
