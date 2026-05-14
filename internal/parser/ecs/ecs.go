// Package ecs renders Amazon ECS EventBridge events into the transport-neutral
// notify.Notification shape. Matches when the EventBridge source is
// "aws.ecs". Two detail-types produce specialized messages — "ECS Task
// State Change" and "ECS Service Action" — and the remaining detail-types
// fall through to a default attachment carrying only the cluster header.
package ecs

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
	name      = "ecs"
	sourceECS = "ecs"

	detailTaskStateChange = "ECS Task State Change"
	detailServiceAction   = "ECS Service Action"

	groupServicePrefix = "service:"
	resourceTaskPrefix = "task/"
	resourceSvcPrefix  = "service/"
	clusterPrefix      = "cluster/"

	defaultStoppedReason = "Unknown"

	authorPrefix = "Amazon ECS - "
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

// Parse renders the Notification for the given ECS EventBridge event. The
// two specialized detail-types build full payloads; everything else
// (Deployment State Change, Container Instance State Change, …) falls
// through to a default Notification carrying just the cluster header.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*notify.Notification, error) {
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
func parseTaskStateChange(e *envelope.Event) (*notify.Notification, error) {
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

	severity := taskSeverity(d.LastStatus, d.DesiredStatus)
	stoppedReason := defaultStoppedReason
	if d.DesiredStatus == "STOPPED" && d.StoppedReason != "" {
		stoppedReason = d.StoppedReason
	}

	subtitle := authorPrefix + detailTaskStateChange
	fields := []notify.Field{
		{Key: "Task", Value: notify.Link(taskURL, task)},
		{Key: "Status", Value: d.LastStatus},
		{Key: "Desired Status", Value: d.DesiredStatus},
		{Key: "Reason", Value: stoppedReason},
		{Key: "Service Logs", Value: notify.Link(logsURL, "View Logs")},
		{Key: "Service", Value: notify.Link(serviceURL, service)},
		{Key: "Cluster", Value: notify.Link(clusterURL, cluster)},
	}

	fallback := fmt.Sprintf("%s - %s - %s", cluster, detailTaskStateChange, d.LastStatus)
	return &notify.Notification{
		Source:   name,
		Severity: severity,
		Title:    subtitle,
		Fields:   fields,
		Fallback: fallback,
	}, nil
}

// parseServiceAction renders the body for an ECS Service Action event.
func parseServiceAction(e *envelope.Event) (*notify.Notification, error) {
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

	severity := serviceSeverity(d.EventType)
	subtitle := authorPrefix + detailServiceAction
	fields := []notify.Field{
		{Key: "Service", Value: notify.Link(serviceURL, service)},
		{Key: "Status", Value: d.EventName},
		{Key: "Service Logs", Value: notify.Link(logsURL, "View Logs")},
		{Key: "Cluster", Value: notify.Link(clusterURL, cluster)},
	}

	fallback := fmt.Sprintf("%s - %s - %s", cluster, detailServiceAction, d.EventName)
	return &notify.Notification{
		Source:   name,
		Severity: severity,
		Title:    subtitle,
		Fields:   fields,
		Fallback: fallback,
	}, nil
}

// parseDefault renders the fall-through shape for ECS detail-types that
// have no specialized renderer (Deployment State Change, Container Instance
// State Change). The output carries just the source and detail-type header.
func parseDefault(e *envelope.Event, detailType string) *notify.Notification {
	cluster, _ := clusterAndRegion(detailClusterARN(e))
	title := fmt.Sprintf("%s - %s", cluster, detailType)
	if cluster == "" {
		title = detailType
	}
	subtitle := authorPrefix + detailType
	return &notify.Notification{
		Source:   name,
		Severity: notify.SeverityNotice,
		Title:    title,
		Subtitle: subtitle,
		Fallback: title,
	}
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

// taskSeverity maps the lastStatus / desiredStatus pair to a Severity. Both
// RUNNING is OK; desiredStatus STOPPED is Critical; anything else is Notice.
func taskSeverity(lastStatus, desiredStatus string) notify.Severity {
	switch {
	case lastStatus == "RUNNING" && desiredStatus == "RUNNING":
		return notify.SeverityOK
	case desiredStatus == "STOPPED":
		return notify.SeverityCritical
	default:
		return notify.SeverityNotice
	}
}

// serviceSeverity maps the Service Action eventType to a Severity. INFO is
// OK (announcement of normal service activity), WARN → Warning,
// ERROR → Critical.
func serviceSeverity(eventType string) notify.Severity {
	switch eventType {
	case "INFO":
		return notify.SeverityOK
	case "WARN":
		return notify.SeverityWarning
	case "ERROR":
		return notify.SeverityCritical
	default:
		return notify.SeverityNotice
	}
}
