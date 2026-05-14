// Package inspector2 renders Slack messages for Amazon Inspector v2 EventBridge
// findings. Matches when the EventBridge source is "aws.inspector2" or the
// detail-type equals "Inspector2 Finding".
//
// Only HIGH / CRITICAL findings claim the event — lower severities fail
// Match so the generic parser renders them as raw-event dumps.
// HIGH renders as a warning attachment; CRITICAL as a danger attachment.
// The parser ties into the dedup package to suppress repeat alerts for the
// same (vulnerabilityId, resourceType, resourceFamily) triple within the
// TTL window.
package inspector2

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/console"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/dedup"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

const (
	name = "inspector2"

	sourceInspector2     = "inspector2"
	detailTypeInspector2 = "Inspector2 Finding"

	severityCritical = "CRITICAL"
	severityHigh     = "HIGH"

	resourceECR    = "AWS_ECR_CONTAINER_IMAGE"
	resourceLambda = "AWS_LAMBDA_FUNCTION"
	resourceEC2    = "AWS_EC2_INSTANCE"

	fallbackTruncate    = 160
	descriptionTruncate = 600

	defaultFixAvailable     = "UNKNOWN"
	defaultExploitAvailable = "NO"

	authorName = "Amazon Inspector"

	// lambdaArnPartCount is the number of colon-separated segments in a
	// stable Lambda function ARN (no $LATEST / version qualifier).
	lambdaArnPartCount = 7

	defaultDedupTTLDays = 7
	hoursPerDay         = 24
)

// inspector2ArnRegionRE matches the region segment of an Inspector v2 finding
// ARN.
var inspector2ArnRegionRE = regexp.MustCompile(`^arn:[^:]+:inspector2:([^:]+):`)

// Parser renders Slack messages for Inspector2 findings.
type Parser struct {
	dedup dedup.Deduplicator
	log   *slog.Logger
}

// New returns a Parser without dedup. Tests that don't need to assert dedup
// behavior use this ctor.
func New() *Parser { return &Parser{} }

// NewWithDedup returns a Parser wired to the given Deduplicator. Pass a nil
// Deduplicator to disable dedup entirely.
func NewWithDedup(d dedup.Deduplicator) *Parser {
	return &Parser{dedup: d}
}

// NewFromConfig is the production ctor — builds a DynamoDB-backed dedup
// store from the supplied aws.Config and table name / TTL.
func NewFromConfig(cfg aws.Config, tableName string, ttlDays int) *Parser {
	if tableName == "" {
		return &Parser{}
	}
	if ttlDays <= 0 {
		ttlDays = defaultDedupTTLDays
	}
	store := dedup.NewDynamoDB(cfg, tableName, time.Duration(ttlDays)*time.Hour*hoursPerDay)
	return &Parser{dedup: store}
}

// WithLogger overrides the default slog.Default() logger. Tests inject a
// buffer-backed handler to assert log output.
func (p *Parser) WithLogger(l *slog.Logger) *Parser {
	p.log = l
	return p
}

// logger returns the configured logger, falling back to slog.Default().
func (p *Parser) logger() *slog.Logger {
	if p.log != nil {
		return p.log
	}
	return slog.Default()
}

// Name returns the stable parser identifier.
func (Parser) Name() string { return name }

// Match claims Inspector2 EventBridge findings whose severity is HIGH or
// CRITICAL. Lower-severity findings (LOW / MEDIUM / INFORMATIONAL) fail
// Match so the generic catch-all renders them as a raw dump — the
// specialized renderer is reserved for actionable severities.
func (Parser) Match(e *envelope.Event) bool {
	if e.Source() != sourceInspector2 && e.DetailType() != detailTypeInspector2 {
		return false
	}
	raw := e.Get("detail")
	if len(raw) == 0 {
		return false
	}
	var probe struct {
		Severity string `json:"severity"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return probe.Severity == severityHigh || probe.Severity == severityCritical
}

// finding captures the subset of detail.* fields the parser reads.
type finding struct {
	Severity                    string                      `json:"severity"`
	FindingArn                  string                      `json:"findingArn"`
	PackageVulnerabilityDetails packageVulnerabilityDetails `json:"packageVulnerabilityDetails"`
	Title                       string                      `json:"title"`
	InspectorScore              *float64                    `json:"inspectorScore"`
	FixAvailable                string                      `json:"fixAvailable"`
	ExploitAvailable            string                      `json:"exploitAvailable"`
	Description                 string                      `json:"description"`
	AwsAccountID                string                      `json:"awsAccountId"`
	Resources                   []resource                  `json:"resources"`
}

// packageVulnerabilityDetails carries the vulnerabilityId field the parser
// uses to build the dedup key and the rendered title.
type packageVulnerabilityDetails struct {
	VulnerabilityID string `json:"vulnerabilityId"`
}

// resource is one element of the detail.resources array.
type resource struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Details resourceDetails `json:"details"`
}

// resourceDetails carries the per-resource-type detail blocks the parser
// reads. Only the fields the renderer references are modeled.
type resourceDetails struct {
	AwsEcrContainerImage *ecrContainerImage `json:"awsEcrContainerImage,omitempty"`
	AwsLambdaFunction    *lambdaFunction    `json:"awsLambdaFunction,omitempty"`
	AwsEc2Instance       *ec2Instance       `json:"awsEc2Instance,omitempty"`
}

// ecrContainerImage carries the ECR-specific details the renderer references.
type ecrContainerImage struct {
	RepositoryName string `json:"repositoryName"`
	ImageHash      string `json:"imageHash"`
}

// lambdaFunction carries the Lambda-specific details the renderer references.
type lambdaFunction struct {
	FunctionArn  string `json:"functionArn"`
	FunctionName string `json:"functionName"`
}

// ec2Instance carries the EC2-specific details the renderer references.
type ec2Instance struct {
	ImageID string `json:"imageId"`
}

// Parse renders the Slack message for an Inspector2 finding.
//
// Severities outside {HIGH, CRITICAL} are silenced. Repeated findings within
// the dedup TTL window are silenced. SDK errors from the dedup store fail
// open — the alert is rendered anyway.
func (p *Parser) Parse(ctx context.Context, e *envelope.Event) (*slack.Message, error) {
	f, ok := decode(e)
	if !ok {
		return nil, fmt.Errorf("inspector2: detail block missing or malformed")
	}

	res := firstResource(f.Resources)
	dedupKey := buildDedupKey(f, res)
	region := findingRegion(f.FindingArn, e.Region())

	if p.dedup != nil {
		meta := map[string]string{
			"severity":          f.Severity,
			"fix_available":     valueOrDefault(f.FixAvailable, defaultFixAvailable),
			"exploit_available": valueOrDefault(f.ExploitAvailable, defaultExploitAvailable),
			"finding_arn":       f.FindingArn,
			"account_id":        f.AwsAccountID,
			"region":            region,
		}
		firstSeen, err := p.dedup.TryReserve(ctx, dedupKey, meta)
		if err == nil && !firstSeen {
			p.logger().InfoContext(ctx, "inspector2 alert deduped",
				"dedup_key", dedupKey,
				"finding_arn", f.FindingArn,
				"severity", f.Severity,
				"resource_type", res.Type,
				"region", region,
			)
			return nil, nil
		}
		p.logger().InfoContext(ctx, "inspector2 alert reserved",
			"dedup_key", dedupKey,
			"finding_arn", f.FindingArn,
			"severity", f.Severity,
			"resource_type", res.Type,
			"region", region,
		)
	}

	color := slack.ColorWarning
	if f.Severity == severityCritical {
		color = slack.ColorCritical
	}

	vulnID := pickVulnID(f)
	titleText := vulnID
	if t := strings.TrimSpace(f.Title); t != "" {
		titleText = vulnID + " — " + t
	}
	consoleURL := findingConsoleURL(region, f.FindingArn)

	header := fmt.Sprintf("*%s*\n_%s_", slack.Link(consoleURL, titleText), authorName)
	body := truncate(f.Description, descriptionTruncate)

	blocks := []slack.Block{slack.SectionBlock(header)}
	if body != "" {
		blocks = append(blocks, slack.SectionBlock(body))
	}
	blocks = append(blocks, slack.FieldsSection(buildFields(f, res, region)))

	fallback := vulnID + ": " + truncate(f.Description, fallbackTruncate)
	return slack.NewMessage(color, fallback, blocks...), nil
}

// decode extracts the typed detail block from the inner event message.
func decode(e *envelope.Event) (finding, bool) {
	raw := e.Get("detail")
	if len(raw) == 0 {
		return finding{}, false
	}
	var f finding
	if err := json.Unmarshal(raw, &f); err != nil {
		return finding{}, false
	}
	return f, true
}

// firstResource returns the first element of the resources slice, or a zero
// value when the slice is empty.
func firstResource(resources []resource) resource {
	if len(resources) == 0 {
		return resource{}
	}
	return resources[0]
}

// buildDedupKey produces the `${vulnId}#${type}#${family}` dedup key.
func buildDedupKey(f finding, res resource) string {
	vulnID := pickVulnID(f)
	resourceType := res.Type
	if resourceType == "" {
		resourceType = "UNKNOWN"
	}
	return vulnID + "#" + resourceType + "#" + resourceFamily(res)
}

// pickVulnID returns the most specific identifier available:
// packageVulnerabilityDetails.vulnerabilityId → title → findingArn →
// "unknown".
func pickVulnID(f finding) string {
	if id := strings.TrimSpace(f.PackageVulnerabilityDetails.VulnerabilityID); id != "" {
		return id
	}
	if t := strings.TrimSpace(f.Title); t != "" {
		return t
	}
	if a := strings.TrimSpace(f.FindingArn); a != "" {
		return a
	}
	return "unknown"
}

// resourceFamily returns a per-type identifier used in the dedup key.
//
//   - ECR: repository name (or resource id).
//   - Lambda: the function ARN truncated to the first seven colon-separated
//     segments (drops the version / $LATEST qualifier).
//   - EC2: AMI image id (or resource id).
//   - other: resource id, falling back to "unknown".
func resourceFamily(res resource) string {
	switch res.Type {
	case resourceECR:
		if res.Details.AwsEcrContainerImage != nil && res.Details.AwsEcrContainerImage.RepositoryName != "" {
			return res.Details.AwsEcrContainerImage.RepositoryName
		}
		return fallbackOrUnknown(res.ID)
	case resourceLambda:
		arn := ""
		if res.Details.AwsLambdaFunction != nil {
			arn = res.Details.AwsLambdaFunction.FunctionArn
		}
		if arn == "" {
			arn = res.ID
		}
		parts := strings.Split(arn, ":")
		if len(parts) >= lambdaArnPartCount {
			return strings.Join(parts[:lambdaArnPartCount], ":")
		}
		return arn
	case resourceEC2:
		if res.Details.AwsEc2Instance != nil && res.Details.AwsEc2Instance.ImageID != "" {
			return res.Details.AwsEc2Instance.ImageID
		}
		return fallbackOrUnknown(res.ID)
	default:
		return fallbackOrUnknown(res.ID)
	}
}

// resourceLabel returns the rendered resource line shown in the Resource
// field.
func resourceLabel(res resource) string {
	switch res.Type {
	case resourceECR:
		repo := ""
		digest := ""
		if res.Details.AwsEcrContainerImage != nil {
			repo = res.Details.AwsEcrContainerImage.RepositoryName
			digest = res.Details.AwsEcrContainerImage.ImageHash
		}
		if repo == "" {
			return res.Type
		}
		const shortDigestLen = 19
		shortDigest := digest
		if len(digest) >= shortDigestLen {
			shortDigest = digest[:shortDigestLen]
		}
		out := "ECR " + repo
		if shortDigest != "" {
			out += " (" + shortDigest + ")"
		}
		return out
	case resourceLambda:
		fn := ""
		if res.Details.AwsLambdaFunction != nil {
			fn = res.Details.AwsLambdaFunction.FunctionName
		}
		if fn == "" {
			fn = res.ID
		}
		return "Lambda " + fn
	case resourceEC2:
		ami := ""
		if res.Details.AwsEc2Instance != nil {
			ami = res.Details.AwsEc2Instance.ImageID
		}
		out := "EC2 " + res.ID
		if ami != "" {
			out += " (AMI " + ami + ")"
		}
		return out
	default:
		t := res.Type
		if t == "" {
			t = "UNKNOWN"
		}
		return t + " " + res.ID
	}
}

// buildFields constructs the field rows shown in the rendered alert.
func buildFields(f finding, res resource, region string) []slack.TextObject {
	score := "n/a"
	if f.InspectorScore != nil {
		score = strconv.FormatFloat(*f.InspectorScore, 'f', -1, 64)
	}
	return []slack.TextObject{
		{Type: slack.TextTypeMrkdwn, Text: "*Severity*\n" + f.Severity},
		{Type: slack.TextTypeMrkdwn, Text: "*Score*\n" + score},
		{Type: slack.TextTypeMrkdwn, Text: "*Fix available*\n" + valueOrDefault(f.FixAvailable, defaultFixAvailable)},
		{Type: slack.TextTypeMrkdwn, Text: "*Exploit available*\n" + valueOrDefault(f.ExploitAvailable, defaultExploitAvailable)},
		{Type: slack.TextTypeMrkdwn, Text: "*Resource*\n" + resourceLabel(res)},
		{Type: slack.TextTypeMrkdwn, Text: "*Account*\n" + fmt.Sprintf("%s (%s)", f.AwsAccountID, region)},
	}
}

// findingConsoleURL builds the Inspector v2 console URL pinned to the finding
// ARN. The query parameter is URL-encoded.
func findingConsoleURL(region, findingArn string) string {
	encoded := url.QueryEscape(findingArn)
	path := "inspector/v2/home"
	fragment := "/findings?findingArn=" + encoded
	return console.URLWithFragment(region, path, fragment)
}

// findingRegion extracts the region segment from an Inspector v2 finding ARN,
// falling back to the envelope region when the ARN does not parse.
func findingRegion(arn, fallback string) string {
	if m := inspector2ArnRegionRE.FindStringSubmatch(arn); len(m) == 2 {
		return m[1]
	}
	return fallback
}

// truncate returns the input shortened to max-3 characters with an ellipsis
// appended when truncation occurs. Matches lodash's `_.truncate` default.
func truncate(s string, maxLen int) string {
	const ellipsis = "..."
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= len(ellipsis) {
		return s[:maxLen]
	}
	return s[:maxLen-len(ellipsis)] + ellipsis
}

// valueOrDefault returns the first non-empty argument.
func valueOrDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// fallbackOrUnknown returns the input if non-empty, otherwise "unknown".
func fallbackOrUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
