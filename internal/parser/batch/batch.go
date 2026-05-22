// Package batch renders AWS Batch EventBridge events into the
// transport-neutral notify.Notification shape. Matches when the
// EventBridge source is "aws.batch". The detail block carries the Batch job
// state machine (SUBMITTED → PENDING → RUNNABLE → STARTING → RUNNING →
// SUCCEEDED | FAILED).
package batch

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/console"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

const (
	name        = "batch"
	sourceBatch = "batch"

	statusSucceeded = "SUCCEEDED"
	statusFailed    = "FAILED"
	statusRunnable  = "RUNNABLE"
	statusRunning   = "RUNNING"

	logsPath = "cloudwatch/home"

	authorBatch = "AWS Batch Notification"
)

// Parser handles AWS Batch EventBridge events.
type Parser struct{}

// New returns a Parser ready to register with the router.
func New() *Parser { return &Parser{} }

// Name returns the stable parser identifier.
func (Parser) Name() string { return name }

// Match returns true when the EventBridge source identifies a Batch event.
func (Parser) Match(e *envelope.Event) bool {
	return e.Source() == sourceBatch
}

// detail captures the Batch EventBridge detail block.
type detail struct {
	JobName      string    `json:"jobName"`
	Status       string    `json:"status"`
	StatusReason string    `json:"statusReason"`
	Attempts     []attempt `json:"attempts"`
}

// attempt captures the bits of detail.attempts[] we read.
type attempt struct {
	Container attemptContainer `json:"container"`
}

// attemptContainer captures detail.attempts[].container.
type attemptContainer struct {
	LogStreamName string `json:"logStreamName"`
}

// Parse renders the Notification for a Batch job state change.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*notify.Notification, error) {
	d, ok := decodeDetail(e)
	if !ok {
		return nil, fmt.Errorf("batch: detail block missing or malformed")
	}

	severity, title := severityAndTitle(d.JobName, d.Status)
	region := e.Region()

	fields := make([]notify.Field, 0, 3)
	fields = append(fields, notify.Field{Key: "Status", Value: d.Status})
	if d.StatusReason != "" {
		fields = append(fields, notify.Field{Key: "Reason", Value: d.StatusReason})
	}
	if logStream := firstLogStream(d.Attempts); logStream != "" {
		fragment := fmt.Sprintf("logEventViewer:group=/aws/batch/job;stream=%s;", logStream)
		logsURL := console.URLWithFragment(region, logsPath, fragment)
		fields = append(fields, notify.Field{Key: "Logs", Value: notify.Link(logsURL, "View Logs")})
	}

	fallback := fmt.Sprintf("Batch Job Event %s %s", d.JobName, d.Status)
	return &notify.Notification{
		Source:   name,
		Severity: severity,
		Title:    title,
		Subtitle: authorBatch,
		Fields:   fields,
		Fallback: fallback,
	}, nil
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
	if d.JobName == "" {
		return detail{}, false
	}
	return d, true
}

// firstLogStream returns the first attempt's container logStreamName, or "".
func firstLogStream(attempts []attempt) string {
	if len(attempts) == 0 {
		return ""
	}
	return attempts[0].Container.LogStreamName
}

// severityAndTitle maps the Batch job status to (severity, title). The Batch
// job state machine is SUBMITTED → PENDING → RUNNABLE → STARTING → RUNNING →
// {SUCCEEDED | FAILED}; only the terminal states alter the title — every
// other transition renders the generic title at Notice severity.
func severityAndTitle(jobName, status string) (sev notify.Severity, title string) {
	base := "Batch Job Event " + jobName
	switch status {
	case statusSucceeded:
		return notify.SeverityOK, jobName + " succeeded"
	case statusFailed:
		return notify.SeverityCritical, jobName + " failed"
	case statusRunnable, statusRunning:
		return notify.SeverityNotice, base
	default:
		return notify.SeverityNotice, base
	}
}
