package codecommit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codecommit"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

const (
	repositoryName = "codecommit-repository"

	repoEventReferenceCreated = "referenceCreated"
	repoEventReferenceUpdated = "referenceUpdated"
	repoEventReferenceDeleted = "referenceDeleted"

	refTypeBranch = "branch"
	refTypeTag    = "tag"
)

// RepositoryParser renders Slack messages for CodeCommit Repository State
// Change EventBridge events.
//
// The parser calls CodeCommit GetBranch/GetCommit through an injected Client
// to enrich the message with the commit subject line. SDK errors are logged
// and swallowed — the Slack message renders without the commit subject.
type RepositoryParser struct {
	client Client
	log    *slog.Logger
}

// NewRepository returns a parser with a nil client (no enrichment, suitable
// for tests that don't care about the commit subject).
func NewRepository() *RepositoryParser {
	return &RepositoryParser{log: slog.Default()}
}

// NewRepositoryWithClient is the test seam — inject a fake Client to exercise
// SDK-driven branches without network access.
func NewRepositoryWithClient(c Client) *RepositoryParser {
	return &RepositoryParser{client: c, log: slog.Default()}
}

// NewRepositoryFromConfig is the production ctor — builds the SDK client
// from the supplied aws.Config.
func NewRepositoryFromConfig(cfg aws.Config) *RepositoryParser {
	return &RepositoryParser{client: buildClient(cfg), log: slog.Default()}
}

// Name returns the stable parser identifier.
func (RepositoryParser) Name() string { return repositoryName }

// repositoryDetail captures the subset of detail.* fields the parser reads.
type repositoryDetail struct {
	CallerUserArn  string `json:"callerUserArn"`
	Event          string `json:"event"`
	RepositoryName string `json:"repositoryName"`
	ReferenceName  string `json:"referenceName"`
	ReferenceType  string `json:"referenceType"`
	CommitID       string `json:"commitId"`
}

// Match returns true when the event is an EventBridge CodeCommit repository
// state-change.
func (RepositoryParser) Match(e *envelope.Event) bool {
	return matchesSource(e) && e.DetailType() == detailTypeRepository
}

// Parse renders the Slack message for a CodeCommit repository event.
func (p *RepositoryParser) Parse(ctx context.Context, e *envelope.Event) (*slack.Message, error) {
	d, ok := decodeRepository(e)
	if !ok {
		return nil, fmt.Errorf("codecommit-repository: detail block missing or malformed")
	}

	region := e.Region()
	repoURL := repoConsoleURL(region, d.RepositoryName)
	title := repositoryTitle(d)

	header := fmt.Sprintf("*%s*\n_%s_", slack.Link(repoURL, title), authorBase)
	blocks := []slack.Block{slack.SectionBlock(header)}

	fields := buildRepositoryFields(d)
	if commitMsg := p.fetchCommitMessage(ctx, d); commitMsg != "" {
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*" + fieldCommitMsg + "*\n" + commitMsg,
		})
	}
	if len(fields) > 0 {
		blocks = append(blocks, slack.FieldsSection(fields))
	}

	fallback := fmt.Sprintf("%s: %s", d.RepositoryName, title)
	return slack.NewMessage(slack.ColorNeutral, fallback, blocks...), nil
}

// decodeRepository extracts the typed detail block from the inner event.
func decodeRepository(e *envelope.Event) (repositoryDetail, bool) {
	raw := e.Get("detail")
	if len(raw) == 0 {
		return repositoryDetail{}, false
	}
	var d repositoryDetail
	if err := json.Unmarshal(raw, &d); err != nil {
		return repositoryDetail{}, false
	}
	return d, true
}

// repositoryTitle maps (event, referenceType, repositoryName) to the Slack
// title. Unknown combinations fall back to the bare repository name.
func repositoryTitle(d repositoryDetail) string {
	repo := d.RepositoryName
	switch {
	case d.Event == repoEventReferenceCreated && d.ReferenceType == refTypeBranch:
		return "New branch created in repository " + repo
	case d.Event == repoEventReferenceUpdated && d.ReferenceType == refTypeBranch:
		return "New commit pushed to repository " + repo
	case d.Event == repoEventReferenceDeleted && d.ReferenceType == refTypeBranch:
		return "Deleted branch in repository " + repo
	case d.Event == repoEventReferenceCreated && d.ReferenceType == refTypeTag:
		return "New tag created in repository " + repo
	case d.Event == repoEventReferenceUpdated && d.ReferenceType == refTypeTag:
		return "Tag reference modified in repository " + repo
	case d.Event == repoEventReferenceDeleted && d.ReferenceType == refTypeTag:
		return "Deleted tag in repository " + repo
	default:
		return repo
	}
}

// buildRepositoryFields returns the conditional Repository / Type / Caller
// ARN field rows.
func buildRepositoryFields(d repositoryDetail) []slack.TextObject {
	fields := make([]slack.TextObject, 0, 3)
	if d.RepositoryName != "" {
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*" + fieldRepository + "*\n" + d.RepositoryName,
		})
	}
	if d.ReferenceType != "" {
		label := capitalizeFirst(d.ReferenceType)
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*" + label + "*\n" + d.ReferenceName,
		})
	}
	if d.CallerUserArn != "" {
		fields = append(fields, slack.TextObject{
			Type: slack.TextTypeMrkdwn,
			Text: "*" + fieldCallerARN + "*\n" + d.CallerUserArn,
		})
	}
	return fields
}

// fetchCommitMessage attempts to enrich the message with the commit subject
// line. SDK errors are logged at WARN and swallowed so the alert still
// renders without the commit subject. When no client is configured the
// function returns the empty string.
func (p *RepositoryParser) fetchCommitMessage(ctx context.Context, d repositoryDetail) string {
	if p.client == nil {
		return ""
	}
	commitID := d.CommitID
	if commitID == "" && d.ReferenceType == refTypeBranch && d.Event == repoEventReferenceUpdated {
		branch, err := p.client.GetBranch(ctx, &codecommit.GetBranchInput{
			RepositoryName: stringPtr(d.RepositoryName),
			BranchName:     stringPtr(d.ReferenceName),
		})
		if err != nil {
			p.log.WarnContext(ctx, "codecommit GetBranch failed",
				"err", err,
				"repository", d.RepositoryName,
				"branch", d.ReferenceName,
			)
			return ""
		}
		if branch != nil && branch.Branch != nil && branch.Branch.CommitId != nil {
			commitID = *branch.Branch.CommitId
		}
	}
	if commitID == "" {
		return ""
	}
	out, err := p.client.GetCommit(ctx, &codecommit.GetCommitInput{
		RepositoryName: stringPtr(d.RepositoryName),
		CommitId:       stringPtr(commitID),
	})
	if err != nil {
		p.log.WarnContext(ctx, "codecommit GetCommit failed",
			"err", err,
			"repository", d.RepositoryName,
			"commit_id", commitID,
		)
		return ""
	}
	if out == nil || out.Commit == nil || out.Commit.Message == nil {
		return ""
	}
	return strings.TrimRight(*out.Commit.Message, "\n")
}

// capitalizeFirst uppercases the first byte of an ASCII identifier so
// "branch" / "tag" render as "Branch" / "Tag".
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// stringPtr returns a pointer to its string argument. The SDK input structs
// take *string for nilable fields.
func stringPtr(s string) *string { return &s }
