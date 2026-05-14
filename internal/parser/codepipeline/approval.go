package codepipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

const (
	approvalName = "codepipeline-approval"

	// expiredThreshold is the numHours boundary below which the approval is
	// treated as already expired.
	expiredThreshold = 0.001
	// minutesThreshold is the boundary below which the remaining time is
	// rendered as minutes rather than hours.
	minutesThreshold = 1.0
	// hoursThreshold is the boundary below which the remaining time is
	// rendered as hours rather than days.
	hoursThreshold = 40.0

	msPerHour  = 3600000.0
	minPerHour = 60.0
	hourPerDay = 24.0
)

// ApprovalParser handles CodePipeline manual-approval SNS notifications.
// Matches when the inner SNS message carries both `consoleLink` and
// `approval.pipelineName`.
type ApprovalParser struct{}

// NewApproval returns an Approval-variant parser ready to register with the
// router.
func NewApproval() *ApprovalParser { return &ApprovalParser{} }

// Name returns the stable parser identifier.
func (ApprovalParser) Name() string { return approvalName }

// approvalMessage is the subset of the approval SNS payload the parser reads.
type approvalMessage struct {
	ConsoleLink string         `json:"consoleLink"`
	Approval    approvalDetail `json:"approval"`
}

// approvalDetail is the inner approval block.
type approvalDetail struct {
	PipelineName       string `json:"pipelineName"`
	StageName          string `json:"stageName"`
	ActionName         string `json:"actionName"`
	ExternalEntityLink string `json:"externalEntityLink"`
	ApprovalReviewLink string `json:"approvalReviewLink"`
	CustomData         string `json:"customData"`
	Expires            string `json:"expires"`
}

// decodeApproval extracts the typed payload from the envelope when it carries
// both consoleLink and approval.pipelineName.
func decodeApproval(e *envelope.Event) (approvalMessage, bool) {
	raw := e.Message()
	if len(raw) == 0 || raw[0] != '{' {
		return approvalMessage{}, false
	}
	var m approvalMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return approvalMessage{}, false
	}
	if m.ConsoleLink == "" || m.Approval.PipelineName == "" {
		return approvalMessage{}, false
	}
	return m, true
}

// Match returns true when the SNS message carries the manual-approval
// fields.
func (ApprovalParser) Match(e *envelope.Event) bool {
	_, ok := decodeApproval(e)
	return ok
}

// Parse renders the Notification for a CodePipeline approval request.
func (ApprovalParser) Parse(_ context.Context, e *envelope.Event) (*notify.Notification, error) {
	m, ok := decodeApproval(e)
	if !ok {
		return nil, fmt.Errorf("codepipeline-approval: payload missing consoleLink or approval.pipelineName")
	}

	pipeline := m.Approval.PipelineName
	stage := m.Approval.StageName
	action := m.Approval.ActionName
	customMsg := m.Approval.CustomData

	numHours := approvalNumHours(m.Approval.Expires, e.Time())
	hrs := renderHours(numHours)

	summary := fmt.Sprintf("Approval required %s for %s / %s", hrs, stage, action)
	if customMsg != "" {
		summary += "\n" + customMsg
	}

	subtitle := authorBase
	if accountID := e.AccountID(); accountID != "" {
		subtitle = fmt.Sprintf("%s (%s)", authorBase, accountID)
	}

	return &notify.Notification{
		Source:   approvalName,
		Severity: notify.SeverityWarning,
		Title:    pipeline,
		TitleURL: m.ConsoleLink,
		Subtitle: subtitle,
		Summary:  summary,
		Fields: []notify.Field{
			{Key: "Review URL", Value: m.Approval.ExternalEntityLink},
			{Key: "Approval URL", Value: m.Approval.ApprovalReviewLink},
		},
		Fallback: fmt.Sprintf("%s >> APPROVAL REQUIRED", pipeline),
	}, nil
}

// approvalNumHours computes the remaining time to expiry in hours. Returns 0
// when the expires field is unparseable — the renderer then maps it to the
// expired branch (numHours < 0.001 → 0 → "expired").
func approvalNumHours(expires string, now time.Time) float64 {
	t, err := time.Parse(time.RFC3339, expires)
	if err != nil {
		return 0
	}
	deltaMs := float64(t.Sub(now)) / float64(time.Millisecond)
	return deltaMs / msPerHour
}

// renderHours maps the remaining-time-to-expiry to its rendered phrase.
func renderHours(numHours float64) string {
	switch {
	case numHours < expiredThreshold:
		return fmt.Sprintf("*%d ago!*", ceilInt(numHours))
	case numHours < minutesThreshold:
		return fmt.Sprintf("within *%d minutes*", roundInt(numHours*minPerHour))
	case numHours < hoursThreshold:
		return fmt.Sprintf("within %d hours", ceilInt(numHours))
	default:
		return fmt.Sprintf("within %d days", ceilInt(numHours/hourPerDay))
	}
}

// ceilInt is math.Ceil cast to int, matching the JS `Math.ceil` idiom.
func ceilInt(f float64) int {
	return int(math.Ceil(f))
}

// roundInt is math.Round cast to int, matching the JS `Math.round` idiom
// (which uses round-half-toward-positive-infinity for positive inputs).
func roundInt(f float64) int {
	return int(math.Round(f))
}
