// Package ecs renders Slack messages for Amazon ECS EventBridge events.
// Matches when the EventBridge source is "aws.ecs". Two detail-types
// produce specialized messages — "ECS Task State Change" and "ECS Service
// Action" — and the remaining detail-types fall through to a default
// attachment carrying only the cluster header.
package ecs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/console"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

const (
	name      = "ecs"
	sourceECS = "ecs"

	detailTaskStateChange = "ECS Task State Change"
	detailServiceAction   = "ECS Service Action"

	groupServicePrefix = "service:"
	resourceTaskPrefix = "task/"
	resourceSvcPrefix  = "service/"
	clusterPrefix      = "cluster/"

	defaultStoppedReason = "Unknown"
)

// Parser handles Amazon ECS EventBridge events.
type Parser struct{}

// New returns a Parser ready to register with the router.
func New() *Parser { return &Parser{} }

// Name returns the stable parser identifier.
func (Parser) Name() string { return name }

// Match returns true when the EventBridge source identifies an ECS event.
func (Parser) Match(e *envelope.Event) bool {
	return e.Source() == sourceECS
}

// taskDetail captures the fields the Task State Change branch reads.
type taskDetail struct {
	ClusterArn    string `json:"clusterArn"`
	TaskArn       string `json:"taskArn"`
	LastStatus    string `json:"lastStatus"`
	DesiredStatus string `json:"desiredStatus"`
	Group         string `json:"group"`
	StoppedReason string `json:"stoppedReason"`
}

// serviceDetail captures the fields the Service Action branch reads.
type serviceDetail struct {
	ClusterArn string `json:"clusterArn"`
	EventType  string `json:"eventType"`
	EventName  string `json:"eventName"`
}

// Parse renders the Slack message for the given ECS EventBridge event. The
// two specialized detail-types build full attachment payloads; everything
// else (Deployment State Change, Container Instance State Change, …) falls
// through to a default attachment carrying just the cluster header.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*slack.Message, error) {
	detailType := e.DetailType()
	switch detailType {
	case detailTaskStateChange:
		return parseTaskStateChange(e)
	case detailServiceAction:
		return parseServiceAction(e)
	default:
		return parseDefault(e, detailType), nil
	}
}

// parseTaskStateChange renders the body for an ECS Task State Change event.
func parseTaskStateChange(e *envelope.Event) (*slack.Message, error) {
	var d taskDetail
	if err := unmarshalDetail(e, &d); err != nil {
		return nil, fmt.Errorf("ecs: decode task detail: %w", err)
	}

	cluster, region := clusterAndRegion(d.ClusterArn)
	if cluster == "" {
		return nil, fmt.Errorf("ecs: cluster missing from task arn %q", d.ClusterArn)
	}
	service := strings.TrimPrefix(d.Group, groupServicePrefix)
	task := taskIDFromARN(d.TaskArn, cluster)

	clusterURL := console.URLWithFragment(region, "ecs/home", "/clusters/"+cluster+"/services")
	serviceURL := console.URLWithFragment(region, "ecs/home", "/clusters/"+cluster+"/services/"+service+"/details")
	taskURL := console.URLWithFragment(region, "ecs/home", "/clusters/"+cluster+"/tasks/"+task+"/details")
	logsURL := console.URLWithFragment(region, "ecs/home", "/clusters/"+cluster+"/services/"+service+"/logs")

	color := taskColor(d.LastStatus, d.DesiredStatus)
	stoppedReason := defaultStoppedReason
	if d.DesiredStatus == "STOPPED" && d.StoppedReason != "" {
		stoppedReason = d.StoppedReason
	}

	author := "Amazon ECS - " + detailTaskStateChange
	header := slack.SectionBlock("*" + author + "*")
	fields := slack.FieldsSection([]slack.TextObject{
		{Type: slack.TextTypeMrkdwn, Text: "*Task*\n" + slack.Link(taskURL, task)},
		{Type: slack.TextTypeMrkdwn, Text: "*Status*\n" + d.LastStatus},
		{Type: slack.TextTypeMrkdwn, Text: "*Desired Status*\n" + d.DesiredStatus},
		{Type: slack.TextTypeMrkdwn, Text: "*Reason*\n" + stoppedReason},
		{Type: slack.TextTypeMrkdwn, Text: "*Service Logs*\n" + slack.Link(logsURL, "View Logs")},
		{Type: slack.TextTypeMrkdwn, Text: "*Service*\n" + slack.Link(serviceURL, service)},
		{Type: slack.TextTypeMrkdwn, Text: "*Cluster*\n" + slack.Link(clusterURL, cluster)},
	})

	fallback := fmt.Sprintf("%s - %s - %s", cluster, detailTaskStateChange, d.LastStatus)
	return slack.NewMessage(color, fallback, header, fields), nil
}

// parseServiceAction renders the body for an ECS Service Action event.
func parseServiceAction(e *envelope.Event) (*slack.Message, error) {
	var d serviceDetail
	if err := unmarshalDetail(e, &d); err != nil {
		return nil, fmt.Errorf("ecs: decode service detail: %w", err)
	}

	cluster, region := clusterAndRegion(d.ClusterArn)
	if cluster == "" {
		return nil, fmt.Errorf("ecs: cluster missing from service arn %q", d.ClusterArn)
	}
	service := serviceFromResources(e, cluster)

	clusterURL := console.URLWithFragment(region, "ecs/home", "/clusters/"+cluster+"/services")
	serviceURL := console.URLWithFragment(region, "ecs/home", "/clusters/"+cluster+"/services/"+service+"/details")
	logsURL := console.URLWithFragment(region, "ecs/home", "/clusters/"+cluster+"/services/"+service+"/logs")

	color := serviceColor(d.EventType)
	author := "Amazon ECS - " + detailServiceAction
	header := slack.SectionBlock("*" + author + "*")
	fields := slack.FieldsSection([]slack.TextObject{
		{Type: slack.TextTypeMrkdwn, Text: "*Service*\n" + slack.Link(serviceURL, service)},
		{Type: slack.TextTypeMrkdwn, Text: "*Status*\n" + d.EventName},
		{Type: slack.TextTypeMrkdwn, Text: "*Service Logs*\n" + slack.Link(logsURL, "View Logs")},
		{Type: slack.TextTypeMrkdwn, Text: "*Cluster*\n" + slack.Link(clusterURL, cluster)},
	})

	fallback := fmt.Sprintf("%s - %s - %s", cluster, detailServiceAction, d.EventName)
	return slack.NewMessage(color, fallback, header, fields), nil
}

// parseDefault renders the fall-through shape for ECS detail-types that
// have no specialized renderer (Deployment State Change, Container Instance
// State Change). The output is an attachment carrying just the source and
// detail-type header.
func parseDefault(e *envelope.Event, detailType string) *slack.Message {
	cluster, _ := clusterAndRegion(detailClusterARN(e))
	title := fmt.Sprintf("%s - %s", cluster, detailType)
	if cluster == "" {
		title = detailType
	}
	author := "Amazon ECS - " + detailType
	blocks := []slack.Block{
		slack.SectionBlock(fmt.Sprintf("*%s*\n_%s_", title, author)),
	}
	return slack.NewMessage(slack.ColorNeutral, title, blocks...)
}

// unmarshalDetail decodes the inner EventBridge detail block into the
// caller-supplied struct.
func unmarshalDetail(e *envelope.Event, dst any) error {
	raw := e.Get("detail")
	if len(raw) == 0 {
		return fmt.Errorf("detail block missing")
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("unmarshal detail: %w", err)
	}
	return nil
}

// detailClusterARN reads the clusterArn field from the detail block without a
// typed struct — used by the default-detail-type branch.
func detailClusterARN(e *envelope.Event) string {
	raw := e.Get("detail")
	if len(raw) == 0 {
		return ""
	}
	var obj struct {
		ClusterArn string `json:"clusterArn"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	return obj.ClusterArn
}

// clusterAndRegion parses an ECS cluster ARN and returns (cluster, region).
func clusterAndRegion(arn string) (cluster, region string) {
	if arn == "" {
		return "", ""
	}
	parsed := envelope.ParseARN(arn)
	cluster = strings.TrimPrefix(parsed.Resource, clusterPrefix)
	region = parsed.Region
	return cluster, region
}

// taskIDFromARN extracts the task ID from a task ARN by stripping the
// "task/" prefix and the cluster name.
func taskIDFromARN(arn, cluster string) string {
	parsed := envelope.ParseARN(arn)
	res := strings.TrimPrefix(parsed.Resource, resourceTaskPrefix)
	return strings.TrimPrefix(res, cluster+"/")
}

// serviceFromResources reads the first element of message.resources and
// extracts the service name (the segment after the cluster).
func serviceFromResources(e *envelope.Event, cluster string) string {
	raw := e.Get("resources")
	if len(raw) == 0 {
		return ""
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil || len(list) == 0 {
		return ""
	}
	parsed := envelope.ParseARN(list[0])
	res := strings.TrimPrefix(parsed.Resource, resourceSvcPrefix)
	return strings.TrimPrefix(res, cluster+"/")
}

// taskColor maps the lastStatus / desiredStatus pair to a Slack color:
// both RUNNING is ok, desiredStatus STOPPED is critical, anything else
// renders as a warning.
func taskColor(lastStatus, desiredStatus string) string {
	switch {
	case lastStatus == "RUNNING" && desiredStatus == "RUNNING":
		return slack.ColorOK
	case desiredStatus == "STOPPED":
		return slack.ColorCritical
	default:
		return slack.ColorWarning
	}
}

// serviceColor maps the Service Action eventType to a Slack color so
// INFO renders ok, WARN yellow, and ERROR red.
func serviceColor(eventType string) string {
	switch eventType {
	case "INFO":
		return slack.ColorOK
	case "WARN":
		return slack.ColorWarning
	case "ERROR":
		return slack.ColorCritical
	default:
		return slack.ColorNeutral
	}
}
