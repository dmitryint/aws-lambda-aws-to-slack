// Package guardduty renders Amazon GuardDuty findings into the
// transport-neutral notify.Notification shape. Events arrive over
// EventBridge with source "aws.guardduty", or over SNS carrying the same
// detail body. Either path is accepted.
//
// The parser branches on detail.service.action.actionType for the body
// (PORT_PROBE, AWS_API_CALL, anything else → JSON dump) and on
// detail.resource.resourceType for the trailer (Instance, AccessKey,
// anything else → JSON dump). Unknown action and resource types render
// with a "${actionType}" / "(<resourceType>)" header followed by a
// pretty-printed JSON dump so archived log lines stay readable.
package guardduty

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

const (
	name           = "guardduty"
	sourceGuardDty = "guardduty"

	actionPortProbe = "PORT_PROBE"
	actionAWSAPI    = "AWS_API_CALL"

	resourceInstance  = "Instance"
	resourceAccessKey = "AccessKey"

	severityMediumGate = 4
	severityHighGate   = 7

	subtitleAmazon = "Amazon GuardDuty"
)

// Parser handles GuardDuty findings from EventBridge or SNS.
type Parser struct{}

// New returns a Parser ready to register with the router.
func New() *Parser { return &Parser{} }

// Name returns the stable parser identifier.
func (Parser) Name() string { return name }

// Match returns true for EventBridge events with source "aws.guardduty" and
// for any payload whose inner message has detail.service.serviceName ==
// "guardduty" (the SNS forwarding shape).
func (Parser) Match(e *envelope.Event) bool {
	if e.Source() == sourceGuardDty {
		return true
	}
	raw := e.Get("detail")
	if len(raw) == 0 {
		return false
	}
	var d struct {
		Service struct {
			ServiceName string `json:"serviceName"`
		} `json:"service"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return false
	}
	return d.Service.ServiceName == sourceGuardDty
}

// finding is the typed view over the detail block the parser reads.
type finding struct {
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Severity    float64          `json:"severity"`
	AccountID   string           `json:"accountId"`
	Region      string           `json:"region"`
	Type        string           `json:"type"`
	Service     findingService   `json:"service"`
	Resource    json.RawMessage  `json:"resource"`
	ResourceTyp findingResHeader `json:"-"`
}

// findingService is the detail.service block.
type findingService struct {
	Action         json.RawMessage `json:"action"`
	AdditionalInfo additionalInfo  `json:"additionalInfo"`
	EventFirstSeen string          `json:"eventFirstSeen"`
	EventLastSeen  string          `json:"eventLastSeen"`
	Count          int             `json:"count"`
}

// additionalInfo carries the threatName / threatListName pair, always
// emitted as a field row even when both are empty.
type additionalInfo struct {
	ThreatName     string `json:"threatName"`
	ThreatListName string `json:"threatListName"`
}

// findingResHeader is the shallow shape used only to pick the resourceType
// discriminator before we dispatch to a more specific decoder.
type findingResHeader struct {
	ResourceType string `json:"resourceType"`
}

// actionEnvelope reads the actionType discriminator off detail.service.action.
type actionEnvelope struct {
	ActionType      string                `json:"actionType"`
	PortProbeAction *portProbeActionBlock `json:"portProbeAction,omitempty"`
	AWSAPICallActn  *awsAPICallActionBlk  `json:"awsApiCallAction,omitempty"`
}

// portProbeActionBlock captures the PORT_PROBE branch.
type portProbeActionBlock struct {
	Blocked          bool                  `json:"blocked"`
	PortProbeDetails []portProbeDetailItem `json:"portProbeDetails"`
}

// portProbeDetailItem is one entry of portProbeDetails.
type portProbeDetailItem struct {
	LocalPortDetails portInfo        `json:"localPortDetails"`
	RemoteIPDetails  remoteIPDetails `json:"remoteIpDetails"`
}

// awsAPICallActionBlk captures the AWS_API_CALL branch.
type awsAPICallActionBlk struct {
	API             string          `json:"api"`
	ServiceName     string          `json:"serviceName"`
	RemoteIPDetails remoteIPDetails `json:"remoteIpDetails"`
}

// remoteIPDetails carries the shared remote IP + geo metadata.
type remoteIPDetails struct {
	IPAddressV4  string          `json:"ipAddressV4"`
	Organization remoteIPOrg     `json:"organization"`
	Country      remoteIPCountry `json:"country"`
	City         remoteIPCity    `json:"city"`
}

// remoteIPOrg matches the organization block.
type remoteIPOrg struct {
	ISP string `json:"isp"`
	Org string `json:"org"`
}

// remoteIPCountry matches the country block.
type remoteIPCountry struct {
	CountryName string `json:"countryName"`
}

// remoteIPCity matches the city block.
type remoteIPCity struct {
	CityName string `json:"cityName"`
}

// portInfo matches the {port, portName} pair.
type portInfo struct {
	Port     int    `json:"port"`
	PortName string `json:"portName"`
}

// instanceResource captures the Instance resource branch.
type instanceResource struct {
	InstanceDetails struct {
		InstanceID   string        `json:"instanceId"`
		InstanceType string        `json:"instanceType"`
		Tags         []instanceTag `json:"tags"`
	} `json:"instanceDetails"`
}

// instanceTag is one element of instanceDetails.tags.
type instanceTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// accessKeyResource captures the AccessKey resource branch.
type accessKeyResource struct {
	AccessKeyDetails struct {
		AccessKeyID string `json:"accessKeyId"`
		PrincipalID string `json:"principalId"`
		UserType    string `json:"userType"`
		UserName    string `json:"userName"`
	} `json:"accessKeyDetails"`
}

// Parse renders the Notification for a GuardDuty finding.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*notify.Notification, error) {
	f, ok := decodeFinding(e)
	if !ok {
		return nil, fmt.Errorf("guardduty: detail block missing or malformed")
	}

	fields := buildHeaderFields(f)
	fields = append(fields, renderAction(f.Service.Action)...)
	fields = append(fields, renderCountFields(f.Service)...)
	fields = append(fields, notify.Field{Key: "Resource Type", Value: f.ResourceTyp.ResourceType})
	fields = append(fields, renderResource(f.ResourceTyp.ResourceType, f.Resource)...)

	severity := severityFor(f.Severity)
	fallback := fmt.Sprintf("%s %s", f.Title, f.Description)
	return &notify.Notification{
		Source:   name,
		Severity: severity,
		Title:    f.Title,
		Subtitle: subtitleAmazon,
		Fields:   fields,
		Fallback: fallback,
	}, nil
}

// decodeFinding pulls the typed view over the EventBridge detail block.
func decodeFinding(e *envelope.Event) (finding, bool) {
	raw := e.Get("detail")
	if len(raw) == 0 {
		return finding{}, false
	}
	var f finding
	if err := json.Unmarshal(raw, &f); err != nil {
		return finding{}, false
	}
	if len(f.Resource) > 0 {
		_ = json.Unmarshal(f.Resource, &f.ResourceTyp)
	}
	return f, true
}

// buildHeaderFields emits Description / Account / Region / Type / Severity /
// (threatName, threatListName). The threat row is always pushed even when
// both values are empty.
func buildHeaderFields(f finding) []notify.Field {
	fields := make([]notify.Field, 0, 6)
	fields = append(fields,
		notify.Field{Key: "Description", Value: f.Description},
		notify.Field{Key: "Account", Value: f.AccountID},
		notify.Field{Key: "Region", Value: f.Region},
		notify.Field{Key: "Type", Value: f.Type},
		notify.Field{Key: "Severity", Value: formatSeverity(f.Severity)},
		notify.Field{Key: f.Service.AdditionalInfo.ThreatName, Value: f.Service.AdditionalInfo.ThreatListName},
	)
	return fields
}

// renderAction dispatches the action-type branches.
func renderAction(raw json.RawMessage) []notify.Field {
	var env actionEnvelope
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &env)
	}
	switch env.ActionType {
	case actionPortProbe:
		return renderPortProbe(env.PortProbeAction)
	case actionAWSAPI:
		return renderAWSAPICall(env.AWSAPICallActn)
	default:
		return renderUnknownAction(raw)
	}
}

// renderPortProbe builds the port-probe fields, reading only
// portProbeDetails[0] and pushing empty strings when the slice is empty.
func renderPortProbe(blk *portProbeActionBlock) []notify.Field {
	var first portProbeDetailItem
	blocked := false
	if blk != nil {
		blocked = blk.Blocked
		if len(blk.PortProbeDetails) > 0 {
			first = blk.PortProbeDetails[0]
		}
	}
	port := first.LocalPortDetails
	remote := first.RemoteIPDetails
	return []notify.Field{
		{Key: "Port probe details", Value: fmt.Sprintf("port %s - %s", formatInt(port.Port), port.PortName)},
		{Key: "Remote probe origin", Value: fmt.Sprintf("%s\n%s - %s",
			remote.IPAddressV4, remote.Organization.ISP, remote.Organization.Org)},
		{Key: "Blocked", Value: fmt.Sprintf("%t", blocked)},
	}
}

// renderAWSAPICall builds the AWS_API_CALL fields.
func renderAWSAPICall(blk *awsAPICallActionBlk) []notify.Field {
	if blk == nil {
		blk = &awsAPICallActionBlk{}
	}
	return []notify.Field{
		{Key: "Service", Value: fmt.Sprintf("%s - %s", blk.ServiceName, blk.API)},
		{Key: "API origin", Value: fmt.Sprintf("%s\n%s - %s",
			blk.RemoteIPDetails.IPAddressV4,
			blk.RemoteIPDetails.Organization.ISP,
			blk.RemoteIPDetails.Organization.Org)},
		{Key: "Location", Value: fmt.Sprintf("%s - %s",
			blk.RemoteIPDetails.Country.CountryName,
			blk.RemoteIPDetails.City.CityName)},
	}
}

// renderUnknownAction emits the catch-all field for action types that
// have no specialized renderer. The title carries the literal
// "${actionType}" sequence verbatim; the body is a pretty-printed JSON
// dump of detail.service.action.
func renderUnknownAction(raw json.RawMessage) []notify.Field {
	pretty := prettyJSON(raw)
	return []notify.Field{{Key: "Unknown Action Type (${actionType})", Value: pretty}}
}

// renderCountFields adds the first/last/event-count rows when count > 1.
func renderCountFields(svc findingService) []notify.Field {
	if svc.Count <= 1 {
		return nil
	}
	return []notify.Field{
		{Key: "First Event Time", Value: svc.EventFirstSeen},
		{Key: "Last Event Time", Value: svc.EventLastSeen},
		{Key: "Event count", Value: formatInt(svc.Count)},
	}
}

// renderResource emits resource-specific rows.
func renderResource(resourceType string, raw json.RawMessage) []notify.Field {
	switch resourceType {
	case resourceInstance:
		return renderInstance(raw)
	case resourceAccessKey:
		return renderAccessKey(raw)
	default:
		return renderUnknownResource(resourceType, raw)
	}
}

// renderInstance renders the Instance resource trailer.
func renderInstance(raw json.RawMessage) []notify.Field {
	var r instanceResource
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &r)
	}
	out := make([]notify.Field, 0, 2+len(r.InstanceDetails.Tags))
	out = append(out,
		notify.Field{Key: "Instance ID", Value: r.InstanceDetails.InstanceID},
		notify.Field{Key: "Instance Type", Value: r.InstanceDetails.InstanceType},
	)
	for _, tag := range r.InstanceDetails.Tags {
		out = append(out, notify.Field{Key: tag.Key, Value: tag.Value})
	}
	return out
}

// renderAccessKey renders the AccessKey resource trailer.
func renderAccessKey(raw json.RawMessage) []notify.Field {
	var r accessKeyResource
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &r)
	}
	return []notify.Field{
		{Key: "AccessKeyId", Value: r.AccessKeyDetails.AccessKeyID},
		{Key: "PrincipalId", Value: r.AccessKeyDetails.PrincipalID},
		{Key: "User Type", Value: r.AccessKeyDetails.UserType},
		{Key: "User Name", Value: r.AccessKeyDetails.UserName},
	}
}

// renderUnknownResource emits the catch-all field for unrecognized resource
// types — the full resource block is dumped as pretty JSON. This covers
// S3Bucket, EKSCluster, RDSDBInstance, Lambda, Container, and any future
// resource type GuardDuty introduces.
func renderUnknownResource(resourceType string, raw json.RawMessage) []notify.Field {
	pretty := prettyJSON(raw)
	return []notify.Field{{Key: "Unknown Resource Type (" + resourceType + ")", Value: pretty}}
}

// severityFor maps the GuardDuty severity float to a Severity. The
// comparisons are strict `>` (`severity > 4` warning; `severity > 7`
// critical), so 4 stays Notice and 7 stays Warning.
func severityFor(severity float64) notify.Severity {
	if severity > severityHighGate {
		return notify.SeverityCritical
	}
	if severity > severityMediumGate {
		return notify.SeverityWarning
	}
	return notify.SeverityNotice
}

// formatSeverity formats a severity value, dropping the trailing ".0" on
// whole numbers so 5 prints as "5", not "5.0".
func formatSeverity(s float64) string {
	if s == float64(int64(s)) {
		return fmt.Sprintf("%d", int64(s))
	}
	return fmt.Sprintf("%g", s)
}

// formatInt returns the decimal representation of an int. Extracted so the
// PORT_PROBE / AWS_API_CALL / count branches share one renderer.
func formatInt(n int) string { return fmt.Sprintf("%d", n) }

// prettyJSON pretty-prints raw JSON with two-space indent. Falls back to
// the raw bytes when re-encoding fails so we never lose data on
// unexpected input.
func prettyJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(out)
}
