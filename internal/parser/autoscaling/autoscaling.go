// Package autoscaling renders Slack messages for AWS Auto Scaling SNS
// notifications. Matches when the inner message carries an
// "AutoScalingGroupARN" field.
package autoscaling

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/console"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

const name = "autoscaling"

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

// Parse renders the Slack message for an Auto Scaling notification.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*slack.Message, error) {
	m, ok := decode(e)
	if !ok {
		return nil, fmt.Errorf("autoscaling: payload missing AutoScalingGroupARN")
	}

	region := regionFromARN(m.AutoScalingGroupARN)
	signInURL := fmt.Sprintf("https://%s.signin.aws.amazon.com/console/ec2?region=%s", m.AccountID, region)
	consoleURL := console.URLWithFragment(region, "ec2/autoscaling/home", "AutoScalingGroups:id="+m.AutoScalingGroupName)

	titleText := fmt.Sprintf("%s - %s", m.AutoScalingGroupName, m.Event)
	title := slack.Link(consoleURL, titleText)
	author := slack.Link(signInURL, fmt.Sprintf("AWS AutoScaling (%s - %s)", region, m.AccountID))
	bodyText := fmt.Sprintf("Auto Scaling triggered %s for service %s.", m.Event, m.Service)

	blocks := []slack.Block{
		slack.SectionBlock(fmt.Sprintf("*%s*\n_%s_", title, author)),
		slack.SectionBlock(bodyText),
		slack.FieldsSection([]slack.TextObject{
			{Type: slack.TextTypeMrkdwn, Text: "*Service*\n" + m.Service},
			{Type: slack.TextTypeMrkdwn, Text: "*Event*\n" + m.Event},
		}),
	}

	fallback := bodyText
	return slack.NewMessage(slack.ColorNeutral, fallback, blocks...), nil
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
