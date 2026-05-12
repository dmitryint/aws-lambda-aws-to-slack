// Package inspector renders Slack messages for the classic Amazon Inspector
// SNS notification feed. Matches when the inner SNS message has a `template`
// field whose value starts with "arn:aws:inspector".
//
// Inspector classic emits five event flavors over SNS — ASSESSMENT_RUN_STARTED,
// ASSESSMENT_RUN_COMPLETED, ASSESSMENT_RUN_STATE_CHANGED, FINDING_REPORTED,
// and ENABLE_ASSESSMENT_NOTIFICATIONS. The last is silenced (Match returns
// true but Parse returns (nil, nil)) — those messages are superfluous setup
// notifications.
package inspector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

const (
	name           = "inspector"
	templatePrefix = "arn:aws:inspector"

	eventAssessmentRunStarted      = "ASSESSMENT_RUN_STARTED"
	eventAssessmentRunCompleted    = "ASSESSMENT_RUN_COMPLETED"
	eventAssessmentRunStateChanged = "ASSESSMENT_RUN_STATE_CHANGED"
	eventFindingReported           = "FINDING_REPORTED"
	eventEnableNotifications       = "ENABLE_ASSESSMENT_NOTIFICATIONS"
)

// arnRegionRE captures the region segment from an Inspector ARN.
var arnRegionRE = regexp.MustCompile(`arn:aws:inspector:(.*?):\d+:.*`)

// stateLabels maps newstate values to the human-readable strings emitted
// in the rendered alert. Anything not listed falls through to the raw
// newstate.
var stateLabels = map[string]string{
	"COMPLETED":                      "Completed",
	"CREATED":                        "Created",
	"START_DATA_COLLECTION_PENDING":  "Starting data collection",
	"COLLECTING_DATA":                "Collecting data",
	"STOP_DATA_COLLECTION_PENDING":   "Stopping data collection",
	"DATA_COLLECTED":                 "Data collected",
	"START_EVALUATING_RULES_PENDING": "Start evaluating rules",
	"EVALUATING_RULES":               "Evaluating rules",
}

// ruleMappings translates raw findingsCount entries (keyed by rules-package
// ARN) into a human-readable label. Each readable name owns the slice of
// regional ARNs that produce it.
var ruleMappings = map[string][]string{
	"Common Vulnerabilities and Exposures": {
		"arn:aws:inspector:us-west-2:758058086616:rulespackage/0-9hgA516p",
		"arn:aws:inspector:us-east-1:316112463485:rulespackage/0-gEjTy7T7",
		"arn:aws:inspector:us-west-1:166987590008:rulespackage/0-TKgzoVOa",
		"arn:aws:inspector:ap-south-1:162588757376:rulespackage/0-LqnJE9dO",
		"arn:aws:inspector:ap-southeast-2:454640832652:rulespackage/0-D5TGAxiR",
		"arn:aws:inspector:ap-northeast-2:526946625049:rulespackage/0-PoGHMznc",
		"arn:aws:inspector:ap-northeast-1:406045910587:rulespackage/0-gHP9oWNT",
		"arn:aws:inspector:eu-west-1:357557129151:rulespackage/0-ubA5XvBh",
		"arn:aws:inspector:eu-central-1:537503971621:rulespackage/0-wNqHa8M9",
	},
	"CIS Operating System Security Configuration Benchmarks": {
		"arn:aws:inspector:us-west-2:758058086616:rulespackage/0-H5hpSawc",
		"arn:aws:inspector:us-east-1:316112463485:rulespackage/0-rExsr2X8",
		"arn:aws:inspector:us-west-1:166987590008:rulespackage/0-xUY8iRqX",
		"arn:aws:inspector:ap-south-1:162588757376:rulespackage/0-PSUlX14m",
		"arn:aws:inspector:ap-southeast-2:454640832652:rulespackage/0-Vkd2Vxjq",
		"arn:aws:inspector:ap-northeast-2:526946625049:rulespackage/0-T9srhg1z",
		"arn:aws:inspector:ap-northeast-1:406045910587:rulespackage/0-7WNjqgGu",
		"arn:aws:inspector:eu-west-1:357557129151:rulespackage/0-sJBhCr0F",
		"arn:aws:inspector:eu-central-1:537503971621:rulespackage/0-nZrAVuv8",
	},
	"Security Best Practices": {
		"arn:aws:inspector:us-west-2:758058086616:rulespackage/0-JJOtZiqQ",
		"arn:aws:inspector:us-east-1:316112463485:rulespackage/0-R01qwB5Q",
		"arn:aws:inspector:us-west-1:166987590008:rulespackage/0-byoQRFYm",
		"arn:aws:inspector:ap-south-1:162588757376:rulespackage/0-fs0IZZBj",
		"arn:aws:inspector:ap-southeast-2:454640832652:rulespackage/0-asL6HRgN",
		"arn:aws:inspector:ap-northeast-2:526946625049:rulespackage/0-2WRpmi4n",
		"arn:aws:inspector:ap-northeast-1:406045910587:rulespackage/0-bBUQnxMq",
		"arn:aws:inspector:eu-west-1:357557129151:rulespackage/0-SnojL3Z6",
		"arn:aws:inspector:eu-central-1:537503971621:rulespackage/0-ZujVHEPB",
	},
	"Runtime Behavior Analysis": {
		"arn:aws:inspector:us-west-2:758058086616:rulespackage/0-vg5GGHSD",
		"arn:aws:inspector:us-east-1:316112463485:rulespackage/0-gBONHN9h",
		"arn:aws:inspector:us-west-1:166987590008:rulespackage/0-yeYxlt0x",
		"arn:aws:inspector:ap-south-1:162588757376:rulespackage/0-EhMQZy6C",
		"arn:aws:inspector:ap-southeast-2:454640832652:rulespackage/0-P8Tel2Xj",
		"arn:aws:inspector:ap-northeast-2:526946625049:rulespackage/0-PoYq7lI7",
		"arn:aws:inspector:ap-northeast-1:406045910587:rulespackage/0-knGBhqEu",
		"arn:aws:inspector:eu-west-1:357557129151:rulespackage/0-lLmwe1zd",
		"arn:aws:inspector:eu-central-1:537503971621:rulespackage/0-0GMUM6fg",
	},
}

// Parser handles Inspector classic SNS notifications.
type Parser struct{}

// New returns a Parser ready to register with the router.
func New() *Parser { return &Parser{} }

// Name returns the stable parser identifier.
func (Parser) Name() string { return name }

// message captures the subset of the SNS payload the parser reads.
type message struct {
	Event         string `json:"event"`
	Template      string `json:"template"`
	Target        string `json:"target"`
	Run           string `json:"run"`
	NewState      string `json:"newstate"`
	Finding       string `json:"finding"`
	FindingsCount string `json:"findingsCount"`
}

// decode pulls the typed payload when the inner SNS message is a JSON object.
func decode(e *envelope.Event) (message, bool) {
	raw := e.Message()
	if len(raw) == 0 || raw[0] != '{' {
		return message{}, false
	}
	var m message
	if err := json.Unmarshal(raw, &m); err != nil {
		return message{}, false
	}
	return m, true
}

// Match returns true when the inner SNS message's `template` field starts
// with "arn:aws:inspector".
func (Parser) Match(e *envelope.Event) bool {
	m, ok := decode(e)
	if !ok {
		return false
	}
	return strings.HasPrefix(m.Template, templatePrefix)
}

// Parse renders the Slack message for an Inspector classic notification.
// Returns (nil, nil) for ENABLE_ASSESSMENT_NOTIFICATIONS, which is
// silenced as a superfluous setup event.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*slack.Message, error) {
	m, ok := decode(e)
	if !ok {
		return nil, fmt.Errorf("inspector: payload is not a JSON object")
	}
	if m.Event == eventEnableNotifications {
		return nil, nil
	}

	title, text, color := renderEvent(m)

	blocks := []slack.Block{
		slack.SectionBlock(fmt.Sprintf("*%s*\n_Amazon Inspector_", title)),
	}
	if text != "" {
		blocks = append(blocks, slack.SectionBlock(text))
	}
	blocks = append(blocks, slack.FieldsSection(buildFields(m)))

	fallback := text
	if fallback == "" {
		fallback = title
	}
	return slack.NewMessage(color, fallback, blocks...), nil
}

// renderEvent maps the event type to (title, body text, color).
func renderEvent(m message) (title, text, color string) {
	switch m.Event {
	case eventAssessmentRunStarted:
		return "Assessment run started", "", slack.ColorOK
	case eventAssessmentRunCompleted:
		return "Assessment run summary", renderCompletedText(m), slack.ColorOK
	case eventFindingReported:
		return "Finding reported", m.Finding, slack.ColorWarning
	case eventAssessmentRunStateChanged:
		return "Assessment run", renderStateChange(m.NewState), slack.ColorNeutral
	default:
		return "", "", slack.ColorNeutral
	}
}

// renderCompletedText composes the body for ASSESSMENT_RUN_COMPLETED — a
// "*<url|Findings>*" line followed by one human-readable line per rules
// package referenced in findingsCount.
func renderCompletedText(m message) string {
	var b strings.Builder
	b.WriteString("*")
	b.WriteString(slack.Link(runURL("finding", m.Run), "Findings"))
	b.WriteString("*\n")

	for i, entry := range parseFindingsCount(m.FindingsCount) {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(formatFinding(entry))
	}
	return b.String()
}

// renderStateChange maps a raw newstate to its human label, falling through
// to the raw value when the state is not enumerated.
func renderStateChange(state string) string {
	if label, ok := stateLabels[state]; ok {
		return label
	}
	return state
}

// parseFindingsCount strips braces and splits on commas, leaving each entry
// verbatim for the per-entry formatter to handle.
func parseFindingsCount(raw string) []string {
	if raw == "" {
		return nil
	}
	cleaned := strings.NewReplacer("{", "", "}", "").Replace(raw)
	return strings.Split(cleaned, ",")
}

// formatFinding renders one findingsCount entry as "<rule-name>: <count>",
// substituting the rules-package ARN for its human-readable name via the
// reverse lookup over ruleMappings. Unknown ARNs are passed through.
func formatFinding(entry string) string {
	trimmed := strings.TrimSpace(entry)
	arn, val, found := strings.Cut(trimmed, "=")
	if !found {
		val = "0"
	}
	if val == "" {
		val = "0"
	}
	ruleName := lookupRuleName(arn)
	return fmt.Sprintf("%s: %s", ruleName, val)
}

// lookupRuleName reverses the ruleMappings table — first key whose value
// slice contains the given ARN wins. Falls back to the raw ARN.
func lookupRuleName(arn string) string {
	for ruleName, arns := range ruleMappings {
		for _, candidate := range arns {
			if candidate == arn {
				return ruleName
			}
		}
	}
	return arn
}

// buildFields returns the Target + Run rows. The Run row is omitted when
// the run ARN is empty.
func buildFields(m message) []slack.TextObject {
	fields := []slack.TextObject{{
		Type: slack.TextTypeMrkdwn,
		Text: "*Target*\n" + m.Target,
	}}
	if m.Run != "" {
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*Run*\n" + slack.Link(runURL("run", m.Run), m.Run) + "\n",
		})
	}
	return fields
}

// runURL builds the Inspector classic console URL for the given run ARN.
// The URL encodes a JSON filter that pins the landing page to the supplied
// assessmentRunArn.
func runURL(kind, runArn string) string {
	region := "invalid"
	if m := arnRegionRE.FindStringSubmatch(runArn); len(m) == 2 {
		region = m[1]
	}
	base := fmt.Sprintf("https://console.aws.amazon.com/inspector/home?region=%s#/%s", region, kind)
	filterJSON := fmt.Sprintf(`{"assessmentRunArns":[%q]}`, runArn)
	return base + "?filter=" + url.QueryEscape(filterJSON)
}
