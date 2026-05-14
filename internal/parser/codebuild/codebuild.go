// Package codebuild renders AWS CodeBuild EventBridge events into the
// transport-neutral notify.Notification shape. Matches when the
// EventBridge source is "aws.codebuild". Only "CodeBuild Build State Change"
// produces a message — the "CodeBuild Build Phase Change" detail-type is
// silenced.
package codebuild

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/console"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

const (
	name             = "codebuild"
	sourceCodeBuild  = "codebuild"
	detailBuildState = "CodeBuild Build State Change"
	detailBuildPhase = "CodeBuild Build Phase Change"
	authorBase       = "AWS CodeBuild"
	logsPath         = "cloudwatch/home"

	statusSucceeded  = "SUCCEEDED"
	statusFailed     = "FAILED"
	statusStopped    = "STOPPED"
	statusInProgress = "IN_PROGRESS"
)

// Parser handles AWS CodeBuild EventBridge events.
type Parser struct{}

// New returns a Parser ready to register with the router.
func New() *Parser { return &Parser{} }

// Name returns the stable parser identifier.
func (Parser) Name() string { return name }

// Match returns true when the EventBridge source identifies a CodeBuild
// event, regardless of detail-type. Phase-change events are silenced
// inside Parse rather than rejected here, so every CodeBuild event flows
// through one parser.
func (Parser) Match(e *envelope.Event) bool {
	return e.Source() == sourceCodeBuild
}

// detail captures the CodeBuild EventBridge detail block.
type detail struct {
	BuildStatus string `json:"build-status"`
	ProjectName string `json:"project-name"`
	BuildID     string `json:"build-id"`
}

// Parse renders the Notification for a CodeBuild state-change event.
// Returns (nil, nil) for the Build Phase Change detail-type.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*notify.Notification, error) {
	if e.DetailType() == detailBuildPhase {
		return nil, nil //nolint:nilnil // silence — Build Phase Change is intentionally dropped
	}
	if e.DetailType() != detailBuildState {
		return nil, fmt.Errorf("codebuild: unexpected detail-type %q", e.DetailType())
	}

	d, ok := decodeDetail(e)
	if !ok {
		return nil, fmt.Errorf("codebuild: detail block missing or malformed")
	}

	region := e.Region()
	accountID := e.AccountID()
	project := d.ProjectName
	buildID := lastColonSegment(d.BuildID)

	buildPath := fmt.Sprintf("codesuite/codebuild/projects/%s/build/%s/",
		project, url.QueryEscape(project+":"+buildID))
	buildURL := console.URL(region, buildPath)
	logsFragment := fmt.Sprintf("logEventViewer:group=/aws/codebuild/%s;start=PT5M", project)
	logsURL := console.URLWithFragment(region, logsPath, logsFragment)

	subtitle := authorBase
	if accountID != "" {
		subtitle = fmt.Sprintf("%s (%s)", authorBase, accountID)
	}

	severity := buildSeverity(d.BuildStatus)

	fields := make([]notify.Field, 0, 2)
	if d.BuildStatus != "" {
		fields = append(fields, notify.Field{Key: "Status", Value: d.BuildStatus})
	}
	fields = append(fields, notify.Field{Key: "Logs", Value: notify.Link(logsURL, "View Logs")})

	return &notify.Notification{
		Source:   name,
		Severity: severity,
		Title:    project,
		TitleURL: buildURL,
		Subtitle: subtitle,
		Fields:   fields,
		Fallback: fmt.Sprintf("%s %s", project, d.BuildStatus),
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
	if d.ProjectName == "" {
		return detail{}, false
	}
	return d, true
}

// buildSeverity maps the CodeBuild build-status to a Severity.
func buildSeverity(status string) notify.Severity {
	switch status {
	case statusSucceeded:
		return notify.SeverityOK
	case statusFailed:
		return notify.SeverityCritical
	case statusStopped, statusInProgress:
		return notify.SeverityNotice
	default:
		return notify.SeverityNotice
	}
}

// lastColonSegment returns the substring after the final ':' in s; for inputs
// without a colon it returns s unchanged. Mirrors the JS
// `_.split(buildId, ":").pop()` idiom.
func lastColonSegment(s string) string {
	idx := strings.LastIndex(s, ":")
	if idx < 0 {
		return s
	}
	return s[idx+1:]
}
