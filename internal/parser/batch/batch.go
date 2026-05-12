// Package batch renders Slack messages for AWS Batch EventBridge events.
// Matches when the EventBridge source is "aws.batch". The detail block
// carries the Batch job state machine (SUBMITTED → PENDING → RUNNABLE →
// STARTING → RUNNING → SUCCEEDED | FAILED).
package batch

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/console"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

const (
	name        = "batch"
	sourceBatch = "batch"

	statusSucceeded = "SUCCEEDED"
	statusFailed    = "FAILED"

	logsPath = "cloudwatch/home"
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

// Parse renders the Slack message for a Batch job state change.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*slack.Message, error) {
	d, ok := decodeDetail(e)
	if !ok {
		return nil, fmt.Errorf("batch: detail block missing or malformed")
	}

	color, title := titleAndColor(d.JobName, d.Status)
	region := e.Region()

	fields := make([]slack.TextObject, 0, 3)
	fields = append(fields, slack.TextObject{
		Type: slack.TextTypeMrkdwn,
		Text: "*Status*\n" + d.Status,
	})
	if d.StatusReason != "" {
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*Reason*\n" + d.StatusReason,
		})
	}

	if logStream := firstLogStream(d.Attempts); logStream != "" {
		fragment := fmt.Sprintf(
			"logEventViewer:group=/aws/batch/job;stream=%s;",
			logStream,
		)
		logsURL := console.URLWithFragment(region, logsPath, fragment)
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*Logs*\n" + slack.Link(logsURL, "View Logs"),
		})
	}

	author := "AWS Batch Notification"
	blocks := []slack.Block{
		slack.SectionBlock(fmt.Sprintf("*%s*\n_%s_", title, author)),
		slack.FieldsSection(fields),
	}

	fallback := fmt.Sprintf("Batch Job Event %s %s", d.JobName, d.Status)
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

// titleAndColor maps the Batch job status to (color, title). The Batch job
// state machine is SUBMITTED → PENDING → RUNNABLE → STARTING → RUNNING →
// {SUCCEEDED | FAILED}; only the terminal states alter the title and color
// — every other transition renders the default neutral color with a
// generic title. The Reason field is omitted when statusReason is empty.
func titleAndColor(jobName, status string) (color, title string) {
	base := "Batch Job Event " + jobName
	switch status {
	case statusSucceeded:
		return slack.ColorOK, jobName + " succeeded"
	case statusFailed:
		return slack.ColorCritical, jobName + " failed"
	default:
		return slack.ColorNeutral, base
	}
}
