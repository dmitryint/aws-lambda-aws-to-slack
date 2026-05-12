// Package cloudwatch renders Slack messages for CloudWatch alarm SNS
// notifications. Matches when the inner SNS message carries both `AlarmName`
// and `AlarmDescription`.
//
// Behavior contract:
//
//  1. Chart-render failures log via slog.Error and never swallow.
//  2. CHART_BUCKET_REGION is read separately from AWS_REGION so the
//     presigned-URL host carries the bucket's region.
//  3. When Trigger.Metrics is present the parser uses
//     MetricStat.Metric.Dimensions (the metric-math case); legacy alarms
//     fall back to Trigger.Dimensions.
//  4. The widget JSON is built end-to-end from the alarm payload and
//     validated by unit tests against samples/cloudwatch/*.json.
//
// The chart image goes inside a Block Kit `image` block while the severity
// color stays on the legacy attachment envelope.
package cloudwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/console"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

const (
	name = "cloudwatch"

	stateOK               = "OK"
	stateAlarm            = "ALARM"
	stateInsufficientData = "INSUFFICIENT_DATA"
)

// Parser renders Slack messages for CloudWatch alarm SNS payloads.
type Parser struct {
	pipeline *ChartRenderingPipeline
	log      *slog.Logger
}

// New returns a parser without chart rendering — useful for tests that don't
// exercise the chart pipeline.
func New() *Parser { return &Parser{log: slog.Default()} }

// NewWithPipeline returns a parser wired to the supplied chart rendering
// pipeline. Tests inject fakes; production wires real SDK clients.
func NewWithPipeline(p *ChartRenderingPipeline) *Parser {
	return &Parser{pipeline: p, log: slog.Default()}
}

// NewFromConfig is the production ctor. It constructs the real SDK clients
// from the provided aws.Config and ChartConfig.
//
// When ChartConfig.BucketName is empty the parser returns a no-chart Parser —
// alerts still flow without an embedded image.
func NewFromConfig(cfg aws.Config, chartCfg ChartConfig) *Parser {
	if chartCfg.BucketName == "" {
		return New()
	}
	region := chartCfg.BucketRegion
	if region == "" {
		region = chartCfg.FallbackRegion
	}
	cwClient := cloudwatch.NewFromConfig(cfg, func(o *cloudwatch.Options) {
		if region != "" {
			o.Region = region
		}
	})
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if region != "" {
			o.Region = region
		}
	})
	presigner := s3.NewPresignClient(s3Client)
	return NewWithPipeline(&ChartRenderingPipeline{
		Renderer:  cwClient,
		Uploader:  s3Client,
		Presigner: presignerAdapter{presigner: presigner},
		Config:    chartCfg,
		Logger:    slog.Default(),
	})
}

// Name returns the stable parser identifier.
func (Parser) Name() string { return name }

// alarmMessage models the subset of the inner SNS payload the parser reads.
type alarmMessage struct {
	AlarmName        string  `json:"AlarmName"`
	AlarmDescription string  `json:"AlarmDescription"`
	AWSAccountID     string  `json:"AWSAccountId"`
	OldStateValue    string  `json:"OldStateValue"`
	NewStateValue    string  `json:"NewStateValue"`
	NewStateReason   string  `json:"NewStateReason"`
	Region           string  `json:"Region"`
	StateChangeTime  string  `json:"StateChangeTime"`
	AlarmArn         string  `json:"AlarmArn"`
	Trigger          trigger `json:"Trigger"`
}

// trigger models the Trigger sub-object of a CloudWatch alarm SNS message.
type trigger struct {
	MetricName string        `json:"MetricName"`
	Namespace  string        `json:"Namespace"`
	Statistic  string        `json:"Statistic"`
	Period     int           `json:"Period"`
	Threshold  *float64      `json:"Threshold,omitempty"`
	Dimensions []dimension   `json:"Dimensions"`
	Metrics    []metricEntry `json:"Metrics"`
}

// dimension is a single CloudWatch dimension entry. The lowercase
// `name` / `value` keys come from the SNS payload as published by AWS.
type dimension struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// metricEntry is one element of Trigger.Metrics. Both the metric-stat form
// and the metric-math expression form are supported.
type metricEntry struct {
	ID         string      `json:"Id"`
	Expression string      `json:"Expression"`
	Label      string      `json:"Label"`
	ReturnData *bool       `json:"ReturnData,omitempty"`
	MetricStat *metricStat `json:"MetricStat,omitempty"`
}

// metricStat is the MetricStat sub-object for a metric-stat metric entry.
type metricStat struct {
	Metric metric `json:"Metric"`
	Period int    `json:"Period"`
	Stat   string `json:"Stat"`
}

// metric is the Metric sub-object of MetricStat.
type metric struct {
	Namespace  string      `json:"Namespace"`
	MetricName string      `json:"MetricName"`
	Dimensions []dimension `json:"Dimensions"`
}

// Match returns true when the SNS inner message carries both `AlarmName`
// and `AlarmDescription`.
func (Parser) Match(e *envelope.Event) bool {
	raw := e.Message()
	if len(raw) == 0 || raw[0] != '{' {
		return false
	}
	var probe struct {
		AlarmName        *string `json:"AlarmName"`
		AlarmDescription *string `json:"AlarmDescription"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return probe.AlarmName != nil && probe.AlarmDescription != nil
}

// Parse renders the Slack message for a CloudWatch alarm SNS payload.
func (p *Parser) Parse(ctx context.Context, e *envelope.Event) (*slack.Message, error) {
	m, ok := decodeAlarm(e)
	if !ok {
		return nil, fmt.Errorf("cloudwatch: payload is not a JSON object")
	}

	region := resolveRegion(m.Region, e.Region())
	color := stateColor(m.NewStateValue)

	header := fmt.Sprintf("*%s*\n_AWS CloudWatch Alarm (%s)_",
		slack.Link(alarmConsoleURL(region, m.AlarmName), m.AlarmName),
		m.AWSAccountID,
	)
	text := buildReasonText(m, e.Time(), region)

	blocks := []slack.Block{slack.SectionBlock(header)}
	if text != "" {
		blocks = append(blocks, slack.SectionBlock(text))
	}
	blocks = append(blocks, slack.FieldsSection([]slack.TextObject{
		{Type: slack.TextTypeMrkdwn, Text: "*State Change*\n" + m.OldStateValue + " → " + m.NewStateValue},
		{Type: slack.TextTypeMrkdwn, Text: "*Region*\n" + m.Region},
	}))

	if imgURL := p.pipeline.renderAlarmChart(ctx, m); imgURL != "" {
		blocks = append(blocks, slack.ImageBlock(imgURL, m.AlarmName+" chart"))
	}

	fallback := fmt.Sprintf("%s state is now %s:\n%s",
		m.AlarmName, m.NewStateValue, m.NewStateReason)
	return slack.NewMessage(color, fallback, blocks...), nil
}

// decodeAlarm extracts the typed alarm message from the inner SNS payload.
func decodeAlarm(e *envelope.Event) (alarmMessage, bool) {
	raw := e.Message()
	if len(raw) == 0 || raw[0] != '{' {
		return alarmMessage{}, false
	}
	var m alarmMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return alarmMessage{}, false
	}
	if m.AlarmName == "" {
		return alarmMessage{}, false
	}
	return m, true
}

// stateColor maps a CloudWatch alarm state to a Slack color, defaulting to
// neutral when the state is composite or unknown.
func stateColor(state string) string {
	switch state {
	case stateOK:
		return slack.ColorOK
	case stateAlarm:
		return slack.ColorCritical
	case stateInsufficientData:
		return slack.ColorWarning
	default:
		return slack.ColorNeutral
	}
}

// buildReasonText appends a "(See recent logs)" link to Lambda Errors alarms.
func buildReasonText(m alarmMessage, ts time.Time, region string) string {
	reason := m.NewStateReason
	logsURL := metricsLogsURL(m.Trigger, ts, region)
	if logsURL == "" {
		return reason
	}
	link := slack.Link(logsURL, "See recent logs")
	if link == "" {
		return reason
	}
	return fmt.Sprintf("%s (%s)", reason, link)
}

// alarmConsoleURL returns the CloudWatch alarm console URL.
func alarmConsoleURL(region, alarmName string) string {
	fragment := "alarm:name=" + alarmName
	path := "cloudwatch/home"
	return console.URLWithFragment(region, path, fragment)
}

// presignerAdapter wraps a *s3.PresignClient so it satisfies the local
// Presigner interface (the SDK returns v4.PresignedHTTPRequest; we expose a
// narrower struct).
type presignerAdapter struct {
	presigner *s3.PresignClient
}

// PresignGetObject implements Presigner.
func (a presignerAdapter) PresignGetObject(ctx context.Context, in *s3.GetObjectInput,
	optFns ...func(*s3.PresignOptions)) (*PresignedRequest, error) {
	req, err := a.presigner.PresignGetObject(ctx, in, optFns...)
	if err != nil {
		return nil, fmt.Errorf("presign GetObject: %w", err)
	}
	return &PresignedRequest{URL: req.URL, Method: req.Method}, nil
}
