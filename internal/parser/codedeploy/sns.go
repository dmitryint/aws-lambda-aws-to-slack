// Package codedeploy renders AWS CodeDeploy events into the
// transport-neutral notify.Notification shape. The package owns two parsers
// — one for the SNS notification trigger (this file) and one for the
// EventBridge state-change rule (cloudwatch.go).
package codedeploy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

const snsName = "codedeploy-sns"

// SNSParser handles CodeDeploy SNS-notification trigger events. Matches
// when the inner message carries both `deploymentId` and
// `deploymentGroupName`.
type SNSParser struct{}

// NewSNS returns an SNS-variant parser ready to register with the router.
func NewSNS() *SNSParser { return &SNSParser{} }

// Name returns the stable parser identifier.
func (SNSParser) Name() string { return snsName }

// snsMessage is the subset of the CodeDeploy SNS payload we read.
type snsMessage struct {
	Status              string `json:"status"`
	DeploymentGroupName string `json:"deploymentGroupName"`
	DeploymentID        string `json:"deploymentId"`
	ApplicationName     string `json:"applicationName"`
}

// decodeSNS pulls the typed payload from the envelope when it is a JSON object
// carrying the required CodeDeploy fields.
func decodeSNS(e *envelope.Event) (snsMessage, bool) {
	raw := e.Message()
	if len(raw) == 0 || raw[0] != '{' {
		return snsMessage{}, false
	}
	var m snsMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return snsMessage{}, false
	}
	if m.DeploymentID == "" || m.DeploymentGroupName == "" {
		return snsMessage{}, false
	}
	return m, true
}

// Match returns true when the SNS message carries both deploymentId and
// deploymentGroupName.
func (SNSParser) Match(e *envelope.Event) bool {
	_, ok := decodeSNS(e)
	return ok
}

// Parse renders the Notification for a CodeDeploy SNS notification.
func (SNSParser) Parse(_ context.Context, e *envelope.Event) (*notify.Notification, error) {
	m, ok := decodeSNS(e)
	if !ok {
		return nil, fmt.Errorf("codedeploy-sns: payload missing deploymentId or deploymentGroupName")
	}
	outcome := snsOutcome(m.Status)
	return renderNotification(renderInput{
		source:          snsName,
		region:          e.Region(),
		application:     m.ApplicationName,
		deploymentID:    m.DeploymentID,
		deploymentGroup: m.DeploymentGroupName,
		status:          m.Status,
		titleSuffix:     outcome.titleSuffix,
		severity:        outcome.severity,
	}), nil
}

// snsOutcome maps the SNS-variant status keywords (SUCCEEDED / STOPPED /
// FAILED / CREATED) to their rendered outcome.
func snsOutcome(status string) statusOutcome {
	switch status {
	case "SUCCEEDED":
		return outcomeFinished
	case "STOPPED":
		return outcomeStopped
	case "FAILED":
		return outcomeFailed
	case "CREATED":
		return outcomeStarted
	default:
		return neutralOutcome
	}
}
