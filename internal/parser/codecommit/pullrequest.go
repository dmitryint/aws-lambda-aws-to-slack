package codecommit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

const (
	pullRequestName = "codecommit-pullrequest"

	prEventCreated             = "pullRequestCreated"
	prEventSourceBranchUpdated = "pullRequestSourceBranchUpdated"
	prEventMergeStatusUpdated  = "pullRequestMergeStatusUpdated"
	prEventStatusChanged       = "pullRequestStatusChanged"

	prStatusClosed = "Closed"

	prMergedTrue  = "True"
	prMergedFalse = "False"
)

// PullRequestParser renders Notifications for CodeCommit Pull Request State
// Change EventBridge events.
type PullRequestParser struct{}

// NewPullRequest returns a Parser ready to register with the router.
func NewPullRequest() *PullRequestParser { return &PullRequestParser{} }

// NewPullRequestFromConfig is the production ctor — accepts an aws.Config for
// API parity with the repository parser, even though the pullrequest parser
// itself makes no SDK calls. Tests use NewPullRequest directly.
func NewPullRequestFromConfig(_ aws.Config) *PullRequestParser { return NewPullRequest() }

// Name returns the stable parser identifier.
func (PullRequestParser) Name() string { return pullRequestName }

// pullRequestDetail captures the subset of detail.* fields the parser reads.
type pullRequestDetail struct {
	CallerUserArn     string   `json:"callerUserArn"`
	Event             string   `json:"event"`
	IsMerged          string   `json:"isMerged"`
	PullRequestID     string   `json:"pullRequestId"`
	PullRequestStatus string   `json:"pullRequestStatus"`
	RepositoryNames   []string `json:"repositoryNames"`
	Title             string   `json:"title"`
}

// Match returns true when the event is an EventBridge CodeCommit pull-request
// state-change.
func (PullRequestParser) Match(e *envelope.Event) bool {
	return matchesSource(e) && e.DetailType() == detailTypePullRequest
}

// Parse renders the Notification for a CodeCommit pull-request event.
func (PullRequestParser) Parse(_ context.Context, e *envelope.Event) (*notify.Notification, error) {
	d, ok := decodePullRequest(e)
	if !ok {
		return nil, fmt.Errorf("codecommit-pullrequest: detail block missing or malformed")
	}

	repoName := ""
	if len(d.RepositoryNames) > 0 {
		repoName = d.RepositoryNames[0]
	}

	title, severity := pullRequestTitleAndSeverity(d)
	region := e.Region()
	prURL := pullRequestConsoleURL(region, repoName, d.PullRequestID)

	return &notify.Notification{
		Source:   pullRequestName,
		Severity: severity,
		Title:    title,
		TitleURL: prURL,
		Subtitle: authorBase,
		Fields:   buildPullRequestFields(d, repoName),
		Fallback: fmt.Sprintf("%s: %s", repoName, title),
	}, nil
}

// decodePullRequest extracts the typed detail block from the inner event.
func decodePullRequest(e *envelope.Event) (pullRequestDetail, bool) {
	raw := e.Get("detail")
	if len(raw) == 0 {
		return pullRequestDetail{}, false
	}
	var d pullRequestDetail
	if err := json.Unmarshal(raw, &d); err != nil {
		return pullRequestDetail{}, false
	}
	return d, true
}

// pullRequestTitleAndSeverity maps the (event, status, isMerged) tuple to the
// title and severity. Per the spec, destructive events on closed unmerged
// PRs are Warning (destructive on protected ref); merges and source-branch
// updates are normal commit/PR activity (Notice).
func pullRequestTitleAndSeverity(d pullRequestDetail) (title string, severity notify.Severity) {
	base := fmt.Sprintf("Pull Request #%s", d.PullRequestID)
	switch {
	case d.Event == prEventMergeStatusUpdated && d.PullRequestStatus == prStatusClosed && d.IsMerged == prMergedTrue:
		return base + " was merged", notify.SeverityNotice
	case d.Event == prEventStatusChanged && d.PullRequestStatus == prStatusClosed && d.IsMerged == prMergedFalse:
		return base + " was closed", notify.SeverityWarning
	case d.Event == prEventCreated:
		return base + " was opened", notify.SeverityNotice
	case d.Event == prEventSourceBranchUpdated:
		return base + " source branch was updated", notify.SeverityNotice
	default:
		return base, notify.SeverityNotice
	}
}

// buildPullRequestFields returns the Repository / Pull Request Title /
// Caller ARN field rows, each conditional on the underlying value being
// non-empty.
func buildPullRequestFields(d pullRequestDetail, repoName string) []notify.Field {
	fields := make([]notify.Field, 0, 3)
	if repoName != "" {
		fields = append(fields, notify.Field{Key: fieldRepository, Value: repoName})
	}
	if d.Title != "" {
		fields = append(fields, notify.Field{Key: "Pull Request Title", Value: d.Title})
	}
	if d.CallerUserArn != "" {
		fields = append(fields, notify.Field{Key: fieldCallerARN, Value: d.CallerUserArn})
	}
	return fields
}
