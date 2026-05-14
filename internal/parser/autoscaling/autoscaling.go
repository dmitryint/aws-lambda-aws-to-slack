// Package autoscaling renders AWS Auto Scaling SNS notifications into the
// transport-neutral notify.Notification shape. Matches when the inner
// message carries an "AutoScalingGroupARN" field.
package autoscaling

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/console"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

const (
	name = "autoscaling"

	eventTerminateError = "autoscaling:EC2_INSTANCE_TERMINATE_ERROR"
)

// Parser handles SNS notifications published by AWS Auto Scaling groups.
type Parser struct{}

// New returns a Parser ready to register with the router.
func New() *Parser { return &Parser{} }

// Name returns the stable parser identifier.
func (Parser) Name() string { return name }

// message is the subset of the Auto Scaling SNS payload the parser reads.
type message struct {
	AccountID            string `json:"AccountId"`
	AutoScalingGroupARN  string `json:"AutoScalingGroupARN"`
	AutoScalingGroupName string `json:"AutoScalingGroupName"`
	Service              string `json:"Service"`
	Event                string `json:"Event"`
}

// decode extracts the typed message from the envelope. Returns ok=false when
// the inner payload is not a JSON object carrying an AutoScalingGroupARN.
func decode(e *envelope.Event) (message, bool) {
	raw := e.Message()
	if len(raw) == 0 || raw[0] != '{' {
		return message{}, false
	}
	var m message
	if err := json.Unmarshal(raw, &m); err != nil {
		return message{}, false
	}
	if m.AutoScalingGroupARN == "" {
		return message{}, false
	}
	return m, true
}

// Match returns true when the SNS message carries an AutoScalingGroupARN.
func (Parser) Match(e *envelope.Event) bool {
	_, ok := decode(e)
	return ok
}

// Parse renders the Notification for an Auto Scaling SNS event.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*notify.Notification, error) {
	m, ok := decode(e)
	if !ok {
		return nil, fmt.Errorf("autoscaling: payload missing AutoScalingGroupARN")
	}

	region := regionFromARN(m.AutoScalingGroupARN)
	signInURL := fmt.Sprintf("https://%s.signin.aws.amazon.com/console/ec2?region=%s", m.AccountID, region)
	consoleURL := console.URLWithFragment(region, "ec2/autoscaling/home", "AutoScalingGroups:id="+m.AutoScalingGroupName)

	titleText := fmt.Sprintf("%s - %s", m.AutoScalingGroupName, m.Event)
	subtitle := notify.Link(signInURL, fmt.Sprintf("AWS AutoScaling (%s - %s)", region, m.AccountID))
	summary := fmt.Sprintf("Auto Scaling triggered %s for service %s.", m.Event, m.Service)

	return &notify.Notification{
		Source:   name,
		Severity: severityFor(m.Event),
		Title:    titleText,
		TitleURL: consoleURL,
		Subtitle: subtitle,
		Summary:  summary,
		Fields: []notify.Field{
			{Key: "Service", Value: m.Service},
			{Key: "Event", Value: m.Event},
		},
		Fallback: summary,
	}, nil
}

// severityFor maps the AutoScaling event keyword to a Severity. Termination
// errors are operational alerts; every other lifecycle event is informational.
func severityFor(event string) notify.Severity {
	if event == eventTerminateError {
		return notify.SeverityCritical
	}
	return notify.SeverityNotice
}

// regionFromARN returns the region segment of an Auto Scaling ARN.
// Example ARN: arn:aws:autoscaling:{region}:{account}:autoScalingGroup:...
func regionFromARN(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}
