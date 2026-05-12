// Package codecommit renders Slack messages for AWS CodeCommit EventBridge
// events. The package hosts two parsers that share helpers — pullrequest
// (detail-type "CodeCommit Pull Request State Change") and repository
// (detail-type "CodeCommit Repository State Change").
package codecommit

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codecommit"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/console"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
)

const (
	sourceCodeCommit = "codecommit"

	detailTypePullRequest = "CodeCommit Pull Request State Change"
	detailTypeRepository  = "CodeCommit Repository State Change"

	authorBase = "AWS CodeCommit"

	fieldRepository = "Repository"
	fieldCallerARN  = "Caller ARN"
	fieldCommitMsg  = "Commit Message"
)

// Client is the seam tests use to inject a fake CodeCommit SDK client. Both
// the repository parser and the pullrequest parser depend on this interface.
type Client interface {
	GetCommit(ctx context.Context, in *codecommit.GetCommitInput,
		optFns ...func(*codecommit.Options)) (*codecommit.GetCommitOutput, error)
	GetBranch(ctx context.Context, in *codecommit.GetBranchInput,
		optFns ...func(*codecommit.Options)) (*codecommit.GetBranchOutput, error)
}

// matchesSource returns true when the EventBridge source identifies a
// CodeCommit event. Both pullrequest and repository parsers gate on this.
func matchesSource(e *envelope.Event) bool {
	return e.Source() == sourceCodeCommit
}

// repoConsoleURL returns the CodeCommit console URL for the repository
// landing page.
func repoConsoleURL(region, repoName string) string {
	return console.URLWithFragment(region, "codecommit/home", "/repository/"+repoName)
}

// pullRequestConsoleURL returns the CodeCommit console URL for the pull
// request landing page.
func pullRequestConsoleURL(region, repoName, pullRequestID string) string {
	return console.URLWithFragment(region, "codecommit/home",
		"/repository/"+repoName+"/pull-request/"+pullRequestID)
}

// buildClient constructs the production CodeCommit SDK client from the given
// aws.Config. It is exported only via the per-parser ctors that accept the
// config.
func buildClient(cfg aws.Config) *codecommit.Client {
	return codecommit.NewFromConfig(cfg)
}
