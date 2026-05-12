// Package codebuild renders Slack messages for AWS CodeBuild EventBridge
// events. Matches when the EventBridge source is "aws.codebuild". Only
// "CodeBuild Build State Change" produces a message — the "CodeBuild
// Build Phase Change" detail-type is silenced.
package codebuild

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/console"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

const (
	name             = "codebuild"
	sourceCodeBuild  = "codebuild"
	detailBuildState = "CodeBuild Build State Change"
	detailBuildPhase = "CodeBuild Build Phase Change"
	authorBase       = "AWS CodeBuild"
	logsPath         = "cloudwatch/home"
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

// Parse renders the Slack message for a CodeBuild state-change event.
// Returns (nil, nil) for the Build Phase Change detail-type.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*slack.Message, error) {
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

	author := authorBase
	if accountID != "" {
		author = fmt.Sprintf("%s (%s)", authorBase, accountID)
	}

	title := slack.Link(buildURL, project)
	color := buildColor(d.BuildStatus)

	fields := make([]slack.TextObject, 0, 2)
	if d.BuildStatus != "" {
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*Status*\n" + d.BuildStatus,
		})
	}
	fields = append(fields, slack.TextObject{
		Type: slack.TextTypeMrkdwn,
		Text: "*Logs*\n" + slack.Link(logsURL, "View Logs"),
	})

	blocks := []slack.Block{
		slack.SectionBlock(fmt.Sprintf("*%s*\n_%s_", title, author)),
		slack.FieldsSection(fields),
	}

	fallback := fmt.Sprintf("%s %s", project, d.BuildStatus)
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
	if d.ProjectName == "" {
		return detail{}, false
	}
	return d, true
}

// buildColor maps the CodeBuild build-status to a Slack color.
func buildColor(status string) string {
	switch status {
	case "SUCCEEDED":
		return slack.ColorOK
	case "STOPPED":
		return slack.ColorWarning
	case "FAILED":
		return slack.ColorCritical
	default:
		return slack.ColorNeutral
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
