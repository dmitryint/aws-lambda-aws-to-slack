package codecommit

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codecommit"
	"github.com/aws/aws-sdk-go-v2/service/codecommit/types"
	"github.com/google/go-cmp/cmp"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

var updateGoldens = flag.Bool("update", false, "rewrite golden files instead of comparing")

const (
	pullRequestSamples = "../../../samples/codecommit/pullrequest"
	repositorySamples  = "../../../samples/codecommit/repository"
	pullRequestGoldens = "testdata/golden/pullrequest"
	repositoryGoldens  = "testdata/golden/repository"
)

func TestPullRequest_Name(t *testing.T) {
	if got := NewPullRequest().Name(); got != "codecommit-pullrequest" {
		t.Fatalf("Name = %q", got)
	}
}

func TestRepository_Name(t *testing.T) {
	if got := NewRepository().Name(); got != "codecommit-repository" {
		t.Fatalf("Name = %q", got)
	}
}

func TestPullRequest_Match(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "right-source-and-type", raw: `{"source":"aws.codecommit","detail-type":"CodeCommit Pull Request State Change"}`, want: true},
		{name: "wrong-detail-type", raw: `{"source":"aws.codecommit","detail-type":"CodeCommit Repository State Change"}`, want: false},
		{name: "wrong-source", raw: `{"source":"aws.ec2","detail-type":"CodeCommit Pull Request State Change"}`, want: false},
		{name: "empty", raw: `{}`, want: false},
	}
	p := NewPullRequest()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev, err := envelope.New(json.RawMessage(tc.raw))
			if err != nil {
				t.Fatalf("envelope.New: %v", err)
			}
			if got := p.Match(ev); got != tc.want {
				t.Fatalf("Match = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRepository_Match(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "right-source-and-type", raw: `{"source":"aws.codecommit","detail-type":"CodeCommit Repository State Change"}`, want: true},
		{name: "wrong-detail-type", raw: `{"source":"aws.codecommit","detail-type":"CodeCommit Pull Request State Change"}`, want: false},
		{name: "wrong-source", raw: `{"source":"aws.ec2","detail-type":"CodeCommit Repository State Change"}`, want: false},
	}
	p := NewRepository()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev, err := envelope.New(json.RawMessage(tc.raw))
			if err != nil {
				t.Fatalf("envelope.New: %v", err)
			}
			if got := p.Match(ev); got != tc.want {
				t.Fatalf("Match = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPullRequest_TitleAndSeverity(t *testing.T) {
	cases := []struct {
		event, status, merged, title string
		severity                     notify.Severity
	}{
		{event: "pullRequestCreated", title: "Pull Request #1 was opened", severity: notify.SeverityNotice},
		{event: "pullRequestSourceBranchUpdated", title: "Pull Request #1 source branch was updated", severity: notify.SeverityNotice},
		{event: "pullRequestMergeStatusUpdated", status: "Closed", merged: "True", title: "Pull Request #1 was merged", severity: notify.SeverityNotice},
		{event: "pullRequestStatusChanged", status: "Closed", merged: "False", title: "Pull Request #1 was closed", severity: notify.SeverityWarning},
		{event: "commentOnPullRequestCreated", title: "Pull Request #1", severity: notify.SeverityNotice},
	}
	for _, tc := range cases {
		title, severity := pullRequestTitleAndSeverity(pullRequestDetail{
			Event:             tc.event,
			PullRequestID:     "1",
			PullRequestStatus: tc.status,
			IsMerged:          tc.merged,
		})
		if title != tc.title || severity != tc.severity {
			t.Fatalf("event %s → (%q, %s), want (%q, %s)", tc.event, title, severity, tc.title, tc.severity)
		}
	}
}

func TestRepository_TitleMapping(t *testing.T) {
	cases := []struct {
		event, refType, repo, want string
	}{
		{event: "referenceCreated", refType: "branch", repo: "r", want: "New branch created in repository r"},
		{event: "referenceUpdated", refType: "branch", repo: "r", want: "New commit pushed to repository r"},
		{event: "referenceDeleted", refType: "branch", repo: "r", want: "Deleted branch in repository r"},
		{event: "referenceCreated", refType: "tag", repo: "r", want: "New tag created in repository r"},
		{event: "referenceUpdated", refType: "tag", repo: "r", want: "Tag reference modified in repository r"},
		{event: "referenceDeleted", refType: "tag", repo: "r", want: "Deleted tag in repository r"},
		{event: "unknown", refType: "branch", repo: "r", want: "r"},
	}
	for _, tc := range cases {
		got := repositoryTitle(repositoryDetail{Event: tc.event, ReferenceType: tc.refType, RepositoryName: tc.repo})
		if got != tc.want {
			t.Fatalf("repositoryTitle(%s,%s) = %q, want %q", tc.event, tc.refType, got, tc.want)
		}
	}
}

func TestCapitalizeFirst(t *testing.T) {
	cases := map[string]string{
		"":       "",
		"branch": "Branch",
		"tag":    "Tag",
		"X":      "X",
	}
	for in, want := range cases {
		if got := capitalizeFirst(in); got != want {
			t.Fatalf("capitalizeFirst(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPullRequest_Parse_ErrorOnMissingDetail(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{"source":"aws.codecommit","detail-type":"CodeCommit Pull Request State Change"}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := NewPullRequest().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error when detail missing")
	}
}

func TestRepository_Parse_ErrorOnMissingDetail(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{"source":"aws.codecommit","detail-type":"CodeCommit Repository State Change"}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := NewRepository().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error when detail missing")
	}
}

// fakeClient is a hand-rolled implementation of Client used by tests to drive
// the SDK seams without touching the network.
type fakeClient struct {
	commitErr  error
	commitMsg  string
	branchErr  error
	branchID   string
	wantCommit string
	wantBranch string
	t          *testing.T
}

func (f *fakeClient) GetCommit(_ context.Context, in *codecommit.GetCommitInput,
	_ ...func(*codecommit.Options)) (*codecommit.GetCommitOutput, error) {
	if f.commitErr != nil {
		return nil, f.commitErr
	}
	if f.wantCommit != "" && in.CommitId != nil && *in.CommitId != f.wantCommit {
		f.t.Fatalf("GetCommit commit id = %q, want %q", *in.CommitId, f.wantCommit)
	}
	msg := f.commitMsg
	return &codecommit.GetCommitOutput{Commit: &types.Commit{Message: &msg}}, nil
}

func (f *fakeClient) GetBranch(_ context.Context, in *codecommit.GetBranchInput,
	_ ...func(*codecommit.Options)) (*codecommit.GetBranchOutput, error) {
	if f.branchErr != nil {
		return nil, f.branchErr
	}
	if f.wantBranch != "" && in.BranchName != nil && *in.BranchName != f.wantBranch {
		f.t.Fatalf("GetBranch branch = %q, want %q", *in.BranchName, f.wantBranch)
	}
	id := f.branchID
	return &codecommit.GetBranchOutput{Branch: &types.BranchInfo{CommitId: &id}}, nil
}

// TestRepository_Parse_WithCommitID_FetchesCommitSubject covers the happy
// path: a commitId is present, GetCommit returns a Message.
func TestRepository_Parse_WithCommitID_FetchesCommitSubject(t *testing.T) {
	ev := readSample(t, filepath.Join(repositorySamples, "ref_updated_branch.json"))
	p := NewRepositoryWithClient(&fakeClient{
		commitMsg:  "Fix typo in README",
		wantCommit: "fedcba9876543210fedcba9876543210fedcba98",
		t:          t,
	})
	msg, err := p.Parse(context.Background(), ev)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if msg == nil {
		t.Fatal("Parse: nil message")
	}
	if !messageContains(msg, "Fix typo in README") {
		t.Fatalf("expected commit subject in message, got %+v", msg)
	}
}

// TestRepository_Parse_GetCommitError_RendersWithoutSubject verifies the
// fail-soft contract: the alert still renders when the CodeCommit SDK
// returns an error.
func TestRepository_Parse_GetCommitError_RendersWithoutSubject(t *testing.T) {
	ev := readSample(t, filepath.Join(repositorySamples, "ref_updated_branch.json"))
	p := NewRepositoryWithClient(&fakeClient{commitErr: errors.New("AccessDenied")})
	msg, err := p.Parse(context.Background(), ev)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if msg == nil {
		t.Fatal("Parse: nil message")
	}
	if messageContains(msg, "Commit Message") {
		t.Fatalf("expected no commit subject when SDK errors, got %+v", msg)
	}
}

// TestRepository_Parse_NoCommitID_BranchUpdated_CallsGetBranchThenGetCommit
// exercises the GetBranch fallback path (commitId is missing on the event but
// the refType is branch and the event is referenceUpdated).
func TestRepository_Parse_NoCommitID_BranchUpdated_CallsGetBranchThenGetCommit(t *testing.T) {
	raw := `{"source":"aws.codecommit","detail-type":"CodeCommit Repository State Change",` +
		`"region":"us-east-1","resources":["arn:aws:codecommit:us-east-1:1:r"],` +
		`"detail":{"event":"referenceUpdated","repositoryName":"r","referenceType":"branch",` +
		`"referenceName":"main","callerUserArn":"arn:aws:iam::1:user/u"}}`
	ev, err := envelope.New(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	p := NewRepositoryWithClient(&fakeClient{
		branchID:   "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		commitMsg:  "Resolved branch",
		wantBranch: "main",
		wantCommit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		t:          t,
	})
	msg, perr := p.Parse(context.Background(), ev)
	if perr != nil {
		t.Fatalf("Parse: %v", perr)
	}
	if !messageContains(msg, "Resolved branch") {
		t.Fatalf("expected GetBranch→GetCommit chain to populate the subject; got %+v", msg)
	}
}

// TestRepository_Parse_GetBranchError_RendersWithoutSubject covers the
// fail-soft path on the GetBranch fallback: the alert renders with no
// commit subject when the SDK call errors.
func TestRepository_Parse_GetBranchError_RendersWithoutSubject(t *testing.T) {
	raw := `{"source":"aws.codecommit","detail-type":"CodeCommit Repository State Change",` +
		`"region":"us-east-1","detail":{"event":"referenceUpdated","repositoryName":"r","referenceType":"branch","referenceName":"main"}}`
	ev, err := envelope.New(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	p := NewRepositoryWithClient(&fakeClient{branchErr: errors.New("not found")})
	msg, perr := p.Parse(context.Background(), ev)
	if perr != nil {
		t.Fatalf("Parse: %v", perr)
	}
	if messageContains(msg, "Commit Message") {
		t.Fatalf("expected no commit subject on GetBranch error, got %+v", msg)
	}
}

// TestPullRequest_SampleGoldens runs each sample under
// samples/codecommit/pullrequest through the parser and compares its output
// against the golden snapshot.
func TestPullRequest_SampleGoldens(t *testing.T) {
	runGoldens(t, NewPullRequest(), pullRequestSamples, pullRequestGoldens)
}

// TestRepository_SampleGoldens runs each sample under
// samples/codecommit/repository through the parser (with a fake SDK client
// that emits a deterministic commit subject) and compares its output against
// the golden snapshot.
func TestRepository_SampleGoldens(t *testing.T) {
	p := NewRepositoryWithClient(&fakeClient{commitMsg: "deterministic commit subject", t: t})
	runGoldens(t, p, repositorySamples, repositoryGoldens)
}

// parserSurface is the minimal interface runGoldens needs from each parser.
type parserSurface interface {
	Match(*envelope.Event) bool
	Parse(context.Context, *envelope.Event) (*notify.Notification, error)
}

// runGoldens drives one parser over every JSON sample in dir, asserting the
// parser claims the sample and matching its rendered output against the
// golden directory.
func runGoldens(t *testing.T, p parserSurface, samplesDir, goldenDir string) {
	t.Helper()
	entries, err := os.ReadDir(samplesDir)
	if err != nil {
		t.Fatalf("read samples %s: %v", samplesDir, err)
	}
	for _, entry := range entries {
		fname := entry.Name()
		if !strings.HasSuffix(fname, ".json") {
			continue
		}
		t.Run(fname, func(t *testing.T) {
			ev := readSample(t, filepath.Join(samplesDir, fname))
			if !p.Match(ev) {
				t.Fatal("Match should be true for sample")
			}
			msg, perr := p.Parse(context.Background(), ev)
			if perr != nil {
				t.Fatalf("Parse: %v", perr)
			}
			compareGolden(t, msg, goldenDir, fname)
		})
	}
}

// readSample loads a JSON fixture and wraps it as an envelope.Event.
func readSample(t *testing.T, path string) *envelope.Event {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	ev, err := envelope.New(raw)
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	return ev.Records()[0]
}

// compareGolden serializes msg to JSON and compares against the golden file.
func compareGolden(t *testing.T, msg any, dir, sampleName string) {
	t.Helper()
	gotJSON, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	goldenPath := filepath.Join(dir, sampleName)
	if *updateGoldens {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir goldens: %v", err)
		}
		if err := os.WriteFile(goldenPath, append(gotJSON, '\n'), 0o600); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to regenerate)", goldenPath, err)
	}
	if diff := cmp.Diff(string(want), string(gotJSON)+"\n"); diff != "" {
		t.Fatalf("golden mismatch %s (-want +got):\n%s", goldenPath, diff)
	}
}

// messageContains is a thin helper that JSON-marshals the message and checks
// for a substring.
func messageContains(msg any, sub string) bool {
	b, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	return strings.Contains(string(b), sub)
}

// TestNewRepositoryFromConfig ensures the production ctor produces a parser
// whose Match still works (a smoke test — we don't call the SDK).
func TestNewRepositoryFromConfig(t *testing.T) {
	p := NewRepositoryFromConfig(aws.Config{})
	if p == nil {
		t.Fatal("nil parser")
	}
	ev, err := envelope.New(json.RawMessage(`{"source":"aws.codecommit","detail-type":"CodeCommit Repository State Change"}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if !p.Match(ev) {
		t.Fatal("Match should be true")
	}
}

// TestNewPullRequestFromConfig is a smoke test on the FromConfig ctor.
func TestNewPullRequestFromConfig(t *testing.T) {
	p := NewPullRequestFromConfig(aws.Config{})
	if p == nil {
		t.Fatal("nil parser")
	}
}
