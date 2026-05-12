package codedeploy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

const (
	cloudWatchName   = "codedeploy-cloudwatch"
	sourceCodeDeploy = "codedeploy"
)

// CloudWatchParser handles CodeDeploy EventBridge state-change events.
// Matches when the EventBridge source is "aws.codedeploy".
type CloudWatchParser struct{}

// NewCloudWatch returns an EventBridge-variant CodeDeploy parser ready to
// register with the router.
func NewCloudWatch() *CloudWatchParser { return &CloudWatchParser{} }

// Name returns the stable parser identifier.
func (CloudWatchParser) Name() string { return cloudWatchName }

// cloudWatchDetail captures the CodeDeploy state-change detail block.
type cloudWatchDetail struct {
	State           string `json:"state"`
	DeploymentGroup string `json:"deploymentGroup"`
	DeploymentID    string `json:"deploymentId"`
	Application     string `json:"application"`
}

// Match returns true when the EventBridge source identifies a CodeDeploy
// event.
func (CloudWatchParser) Match(e *envelope.Event) bool {
	return e.Source() == sourceCodeDeploy
}

// Parse renders the Slack message for a CodeDeploy state-change event.
func (CloudWatchParser) Parse(_ context.Context, e *envelope.Event) (*slack.Message, error) {
	d, ok := decodeCloudWatchDetail(e)
	if !ok {
		return nil, fmt.Errorf("codedeploy-cloudwatch: detail block missing or malformed")
	}
	outcome := cloudWatchOutcome(d.State)
	return renderMessage(renderInput{
		region:          e.Region(),
		application:     d.Application,
		deploymentID:    d.DeploymentID,
		deploymentGroup: d.DeploymentGroup,
		status:          d.State,
		titleSuffix:     outcome.titleSuffix,
		color:           outcome.color,
	}), nil
}

// cloudWatchOutcome maps the EventBridge-variant state keywords (SUCCESS /
// STOP / FAILURE / START) to their rendered outcome.
func cloudWatchOutcome(state string) statusOutcome {
	switch state {
	case "SUCCESS":
		return outcomeFinished
	case "STOP":
		return outcomeStopped
	case "FAILURE":
		return outcomeFailed
	case "START":
		return outcomeStarted
	default:
		return neutralOutcome
	}
}

// decodeCloudWatchDetail extracts the typed detail block from the inner
// EventBridge message.
func decodeCloudWatchDetail(e *envelope.Event) (cloudWatchDetail, bool) {
	raw := e.Get("detail")
	if len(raw) == 0 {
		return cloudWatchDetail{}, false
	}
	var d cloudWatchDetail
	if err := json.Unmarshal(raw, &d); err != nil {
		return cloudWatchDetail{}, false
	}
	if d.DeploymentID == "" || d.Application == "" {
		return cloudWatchDetail{}, false
	}
	return d, true
}
