// Package codepipeline renders Slack messages for AWS CodePipeline events.
// The package owns two parsers — one for pipeline/stage/action state-change
// EventBridge events (pipeline.go) and one for SNS-delivered manual
// approval notifications (approval.go).
package codepipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/console"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

const (
	pipelineName    = "codepipeline"
	sourcePipeline  = "codepipeline"
	missingPipeline = "<missing-pipeline>"
	unknownStage    = "UNKNOWN"
	unknownAction   = "UNKNOWN"
	authorBase      = "AWS CodePipeline"

	executionStage  = "Stage"
	executionAction = "Action"
)

// detailTypeRE captures the execution scope (Pipeline / Stage / Action) from
// the EventBridge detail-type string. The pattern is un-anchored — the
// EventBridge detail-type is "CodePipeline Pipeline/Stage/Action Execution
// State Change", and we want the word immediately before "Execution State
// Change".
var detailTypeRE = regexp.MustCompile(`(\w+) Execution State Change`)

// Parser handles CodePipeline pipeline/stage/action state-change events.
type Parser struct{}

// New returns a Parser ready to register with the router.
func New() *Parser { return &Parser{} }

// Name returns the stable parser identifier.
func (Parser) Name() string { return pipelineName }

// pipelineDetail captures the EventBridge detail fields the parser reads.
type pipelineDetail struct {
	Pipeline    string        `json:"pipeline"`
	State       string        `json:"state"`
	ExecutionID string        `json:"execution-id"`
	Stage       string        `json:"stage"`
	Action      string        `json:"action"`
	Type        *pipelineType `json:"type,omitempty"`
}

// pipelineType captures the action-type nested block (provider / category).
type pipelineType struct {
	Provider string `json:"provider"`
	Category string `json:"category"`
}

// Match returns true when the EventBridge source is "codepipeline" and the
// inner message does NOT carry an approval block — the approval payload is
// claimed by the approval parser. The rejection ensures the approval parser
// wins regardless of waterfall order.
func (Parser) Match(e *envelope.Event) bool {
	if e.Source() != sourcePipeline {
		return false
	}
	approval := e.Get("approval")
	if len(approval) == 0 {
		return true
	}
	var a struct {
		PipelineName string `json:"pipelineName"`
	}
	if err := json.Unmarshal(approval, &a); err != nil {
		return true
	}
	return a.PipelineName == ""
}

// Parse renders the Slack message for a CodePipeline state-change event.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*slack.Message, error) {
	d := decodePipelineDetail(e)
	pipeline := d.Pipeline
	if pipeline == "" {
		pipeline = missingPipeline
	}
	if d.Stage == "" {
		d.Stage = unknownStage
	}
	if d.Action == "" {
		d.Action = unknownAction
	}

	region := e.Region()
	consoleURL := console.URLWithFragment(region, "codepipeline/home", "/view/"+pipeline)

	author := authorBase
	if accountID := e.AccountID(); accountID != "" {
		author = fmt.Sprintf("%s (%s)", authorBase, accountID)
	}

	detailType := e.DetailType()
	scope := executionScope(detailType)
	title, text := renderTitleAndText(scope, pipeline, d, detailType)
	headerLink := slack.Link(consoleURL, title)

	blocks := []slack.Block{
		slack.SectionBlock(fmt.Sprintf("*%s*\n_%s_", headerLink, author)),
		slack.SectionBlock(text),
	}

	color := pipelineColor(d.State)
	fallback := fmt.Sprintf("%s >> %s", pipeline, d.State)
	return slack.NewMessage(color, fallback, blocks...), nil
}

// decodePipelineDetail extracts the typed detail block. Missing fields
// surface as empty strings rather than an error so partial detail blocks
// still render.
func decodePipelineDetail(e *envelope.Event) pipelineDetail {
	raw := e.Get("detail")
	var d pipelineDetail
	if len(raw) == 0 {
		return d
	}
	_ = json.Unmarshal(raw, &d)
	return d
}

// executionScope returns "Pipeline", "Stage", "Action", or "?" for
// detail-types that don't match the expected pattern.
func executionScope(detailType string) string {
	m := detailTypeRE.FindStringSubmatch(detailType)
	if len(m) == 2 {
		return m[1]
	}
	return "?"
}

// renderTitleAndText constructs the per-scope title and body text strings.
func renderTitleAndText(scope, pipeline string, d pipelineDetail, detailType string) (title, text string) {
	switch scope {
	case executionAction:
		provider := ""
		category := ""
		if d.Type != nil {
			provider = d.Type.Provider
			category = d.Type.Category
		}
		title = fmt.Sprintf("%s [Action: %s/%s] >> %s", pipeline, d.Stage, d.Action, d.State)
		text = fmt.Sprintf("ExecID %s is now %s at Action %s/%s (type: %s / %s)",
			d.ExecutionID, d.State, d.Stage, d.Action, provider, category)
	case executionStage:
		title = fmt.Sprintf("%s [Stage: %s] >> %s", pipeline, d.Stage, d.State)
		text = fmt.Sprintf("ExecID %s is now %s at Stage %s", d.ExecutionID, d.State, d.Stage)
	default:
		title = fmt.Sprintf("%s >> %s", pipeline, d.State)
		text = detailType
	}
	return title, text
}

// pipelineColor maps the pipeline state to a Slack color.
func pipelineColor(state string) string {
	switch state {
	case "STARTED":
		return slack.ColorAccent
	case "SUCCEEDED":
		return slack.ColorOK
	case "FAILED":
		return slack.ColorCritical
	case "CANCELED":
		return slack.ColorWarning
	default:
		return slack.ColorNeutral
	}
}
