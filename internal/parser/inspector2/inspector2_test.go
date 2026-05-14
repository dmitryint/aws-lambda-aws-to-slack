package inspector2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/google/go-cmp/cmp"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
)

var updateGoldens = flag.Bool("update", false, "rewrite golden files instead of comparing")

const (
	samplesRoot = "../../../samples/inspector2"
	goldenRoot  = "testdata/golden"
)

func TestInspector2_Name(t *testing.T) {
	if got := New().Name(); got != "inspector2" {
		t.Fatalf("Name = %q", got)
	}
}

func TestInspector2_Match(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "high-by-source", raw: `{"source":"aws.inspector2","detail":{"severity":"HIGH"}}`, want: true},
		{name: "critical-by-detail-type", raw: `{"detail-type":"Inspector2 Finding","detail":{"severity":"CRITICAL"}}`, want: true},
		{name: "medium-rejected", raw: `{"source":"aws.inspector2","detail":{"severity":"MEDIUM"}}`, want: false},
		{name: "low-rejected", raw: `{"source":"aws.inspector2","detail":{"severity":"LOW"}}`, want: false},
		{name: "informational-rejected", raw: `{"source":"aws.inspector2","detail":{"severity":"INFORMATIONAL"}}`, want: false},
		{name: "missing-detail-block", raw: `{"source":"aws.inspector2"}`, want: false},
		{name: "wrong-source-and-type", raw: `{"source":"aws.inspector","detail-type":"X","detail":{"severity":"HIGH"}}`, want: false},
		{name: "empty", raw: `{}`, want: false},
	}
	p := New()
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

// TestInspector2_Match_MediumSampleFallsThrough asserts the canonical
// medium-severity sample fails Match so the router lets the generic
// catch-all render it as a raw dump.
func TestInspector2_Match_MediumSampleFallsThrough(t *testing.T) {
	ev := readEvent(t, filepath.Join(samplesRoot, "finding_medium_silenced.json"))
	if New().Match(ev) {
		t.Fatal("MEDIUM severity must not match — should fall through to generic")
	}
}

func TestInspector2_Parse_HighRendersWarning(t *testing.T) {
	ev := readEvent(t, filepath.Join(samplesRoot, "finding_high_lambda.json"))
	msg, err := New().Parse(context.Background(), ev)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if msg == nil {
		t.Fatal("Parse: nil message")
	}
	if got := msg.Attachments[0].Color; got != "warning" {
		t.Fatalf("color = %q, want warning", got)
	}
}

func TestInspector2_Parse_CriticalRendersDanger(t *testing.T) {
	ev := readEvent(t, filepath.Join(samplesRoot, "finding_critical_ecr.json"))
	msg, err := New().Parse(context.Background(), ev)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if msg == nil {
		t.Fatal("Parse: nil message")
	}
	if got := msg.Attachments[0].Color; got != "danger" {
		t.Fatalf("color = %q, want danger", got)
	}
}

func TestInspector2_Parse_ErrorOnMissingDetail(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{"source":"aws.inspector2"}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := New().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error when detail missing")
	}
}

// fakeDedup is a hand-rolled Deduplicator that records the last call and
// drives the result with the configured firstSeen + err.
type fakeDedup struct {
	firstSeen bool
	err       error
	calls     int
	gotKey    string
	gotMeta   map[string]string
}

func (f *fakeDedup) TryReserve(_ context.Context, key string, meta map[string]string) (bool, error) {
	f.calls++
	f.gotKey = key
	f.gotMeta = meta
	return f.firstSeen, f.err
}

// TestInspector2_Parse_DedupHit_Silences covers the (false, nil) branch from
// the dedup store — the parser must return (nil, nil) without an error.
func TestInspector2_Parse_DedupHit_Silences(t *testing.T) {
	ev := readEvent(t, filepath.Join(samplesRoot, "finding_high_lambda.json"))
	d := &fakeDedup{firstSeen: false}
	msg, err := NewWithDedup(d).Parse(context.Background(), ev)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if msg != nil {
		t.Fatalf("dedup hit should silence the message, got %+v", msg)
	}
	if d.calls != 1 {
		t.Fatalf("dedup calls = %d, want 1", d.calls)
	}
	wantKey := "CVE-2024-LAMBDA#AWS_LAMBDA_FUNCTION#arn:aws:lambda:us-east-1:123456789012:function:example-fn"
	if d.gotKey != wantKey {
		t.Fatalf("dedup key = %q, want %q", d.gotKey, wantKey)
	}
}

// TestInspector2_Parse_DedupHit_LogsDeduped asserts the INFO log line that
// gives operators visibility into dedup decisions in CloudWatch Logs.
func TestInspector2_Parse_DedupHit_LogsDeduped(t *testing.T) {
	ev := readEvent(t, filepath.Join(samplesRoot, "finding_high_lambda.json"))
	d := &fakeDedup{firstSeen: false}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if _, err := NewWithDedup(d).WithLogger(logger).Parse(context.Background(), ev); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	out := buf.String()
	wantKey := "CVE-2024-LAMBDA#AWS_LAMBDA_FUNCTION#arn:aws:lambda:us-east-1:123456789012:function:example-fn"
	for _, sub := range []string{
		`"msg":"inspector2 alert deduped"`,
		`"dedup_key":"` + wantKey + `"`,
		`"severity":"HIGH"`,
		`"resource_type":"AWS_LAMBDA_FUNCTION"`,
	} {
		if !strings.Contains(out, sub) {
			t.Fatalf("log output missing %q\nfull log:\n%s", sub, out)
		}
	}
}

// TestInspector2_Parse_DedupMiss_LogsReserved asserts that a first-sighting
// also emits an INFO log so operators can grep for newly raised alerts.
func TestInspector2_Parse_DedupMiss_LogsReserved(t *testing.T) {
	ev := readEvent(t, filepath.Join(samplesRoot, "finding_high_lambda.json"))
	d := &fakeDedup{firstSeen: true}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if _, err := NewWithDedup(d).WithLogger(logger).Parse(context.Background(), ev); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if !strings.Contains(buf.String(), `"msg":"inspector2 alert reserved"`) {
		t.Fatalf("expected 'alert reserved' info log, got:\n%s", buf.String())
	}
}

// TestInspector2_Parse_DedupMiss_Renders covers the (true, nil) branch.
func TestInspector2_Parse_DedupMiss_Renders(t *testing.T) {
	ev := readEvent(t, filepath.Join(samplesRoot, "finding_high_lambda.json"))
	d := &fakeDedup{firstSeen: true}
	msg, err := NewWithDedup(d).Parse(context.Background(), ev)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if msg == nil {
		t.Fatal("dedup miss should render a message")
	}
}

// TestInspector2_Parse_DedupError_FailsOpen covers the SDK-error branch:
// the alert must render anyway (fail-open on transient dedup errors).
func TestInspector2_Parse_DedupError_FailsOpen(t *testing.T) {
	ev := readEvent(t, filepath.Join(samplesRoot, "finding_high_lambda.json"))
	d := &fakeDedup{err: errors.New("DynamoDB transient")}
	msg, err := NewWithDedup(d).Parse(context.Background(), ev)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if msg == nil {
		t.Fatal("dedup SDK error should fail open and render the message")
	}
}

func TestInspector2_DedupKey_PerResourceType(t *testing.T) {
	cases := []struct {
		sample string
		want   string
	}{
		{
			sample: "finding_critical_ecr.json",
			want:   "CVE-2024-EXAMPLE#AWS_ECR_CONTAINER_IMAGE#example-repo",
		},
		{
			sample: "finding_high_lambda.json",
			want:   "CVE-2024-LAMBDA#AWS_LAMBDA_FUNCTION#arn:aws:lambda:us-east-1:123456789012:function:example-fn",
		},
		{
			sample: "finding_high_ec2.json",
			want:   "CVE-2024-EC2#AWS_EC2_INSTANCE#ami-0123456789abcdef0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.sample, func(t *testing.T) {
			ev := readEvent(t, filepath.Join(samplesRoot, tc.sample))
			d := &fakeDedup{firstSeen: true}
			if _, err := NewWithDedup(d).Parse(context.Background(), ev); err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if d.gotKey != tc.want {
				t.Fatalf("dedup key = %q, want %q", d.gotKey, tc.want)
			}
		})
	}
}

func TestInspector2_PickVulnID(t *testing.T) {
	if got := pickVulnID(finding{}); got != "unknown" {
		t.Fatalf("empty finding → %q, want unknown", got)
	}
	if got := pickVulnID(finding{Title: "title-only"}); got != "title-only" {
		t.Fatalf("title fallback → %q", got)
	}
	if got := pickVulnID(finding{FindingArn: "arn:x"}); got != "arn:x" {
		t.Fatalf("arn fallback → %q", got)
	}
	got := pickVulnID(finding{
		PackageVulnerabilityDetails: packageVulnerabilityDetails{VulnerabilityID: "CVE-1"},
		Title:                       "title",
		FindingArn:                  "arn:x",
	})
	if got != "CVE-1" {
		t.Fatalf("vuln id preferred → %q", got)
	}
}

func TestInspector2_FallbackOrUnknown(t *testing.T) {
	if got := fallbackOrUnknown(""); got != "unknown" {
		t.Fatalf("empty → %q", got)
	}
	if got := fallbackOrUnknown("x"); got != "x" {
		t.Fatalf("passthrough → %q", got)
	}
}

func TestInspector2_ValueOrDefault(t *testing.T) {
	if got := valueOrDefault("", "fallback"); got != "fallback" {
		t.Fatalf("got %q", got)
	}
	if got := valueOrDefault("v", "fallback"); got != "v" {
		t.Fatalf("got %q", got)
	}
}

func TestInspector2_FirstResource_Empty(t *testing.T) {
	if got := firstResource(nil); got.Type != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestInspector2_ResourceLabel_Unknown(t *testing.T) {
	got := resourceLabel(resource{Type: "AWS_S3_BUCKET", ID: "b"})
	if got != "AWS_S3_BUCKET b" {
		t.Fatalf("got %q", got)
	}
	got = resourceLabel(resource{Type: "", ID: "x"})
	if got != "UNKNOWN x" {
		t.Fatalf("got %q", got)
	}
	got = resourceLabel(resource{Type: "AWS_ECR_CONTAINER_IMAGE"})
	if got != "AWS_ECR_CONTAINER_IMAGE" {
		t.Fatalf("ECR without repo → %q", got)
	}
	got = resourceLabel(resource{Type: "AWS_LAMBDA_FUNCTION", ID: "arn:lambda"})
	if got != "Lambda arn:lambda" {
		t.Fatalf("Lambda fallback id → %q", got)
	}
}

func TestInspector2_ResourceFamily_LambdaShortArn(t *testing.T) {
	got := resourceFamily(resource{
		Type: "AWS_LAMBDA_FUNCTION",
		ID:   "arn:aws:lambda:us-east-1:1:function",
	})
	if got != "arn:aws:lambda:us-east-1:1:function" {
		t.Fatalf("short ARN should pass through: %q", got)
	}
}

func TestInspector2_ResourceFamily_DefaultCase(t *testing.T) {
	got := resourceFamily(resource{Type: "AWS_OTHER", ID: "x"})
	if got != "x" {
		t.Fatalf("got %q", got)
	}
	got = resourceFamily(resource{Type: "AWS_OTHER"})
	if got != "unknown" {
		t.Fatalf("got %q", got)
	}
}

func TestInspector2_FindingRegion_Fallback(t *testing.T) {
	if got := findingRegion("not-an-arn", "us-east-1"); got != "us-east-1" {
		t.Fatalf("expected fallback, got %q", got)
	}
	if got := findingRegion("arn:aws:inspector2:eu-west-3:1:finding/x", "us-east-1"); got != "eu-west-3" {
		t.Fatalf("expected eu-west-3, got %q", got)
	}
}

func TestInspector2_Truncate(t *testing.T) {
	cases := []struct {
		in     string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"abcdefghij", 10, "abcdefghij"},
		{"abcdefghijk", 10, "abcdefg..."},
		{"hi", 2, "hi"},
		{"hello", 2, "he"},
	}
	for _, tc := range cases {
		got := truncate(tc.in, tc.maxLen)
		if got != tc.want {
			t.Fatalf("truncate(%q,%d) = %q, want %q", tc.in, tc.maxLen, got, tc.want)
		}
	}
}

func TestInspector2_SampleGoldens(t *testing.T) {
	p := New()
	entries, err := os.ReadDir(samplesRoot)
	if err != nil {
		t.Fatalf("read samples: %v", err)
	}
	for _, entry := range entries {
		fname := entry.Name()
		if !strings.HasSuffix(fname, ".json") {
			continue
		}
		if strings.Contains(fname, "silenced") || strings.Contains(fname, "fallthrough") {
			continue
		}
		t.Run(fname, func(t *testing.T) {
			ev := readEvent(t, filepath.Join(samplesRoot, fname))
			if !p.Match(ev) {
				t.Fatal("Match should be true")
			}
			msg, perr := p.Parse(context.Background(), ev)
			if perr != nil {
				t.Fatalf("Parse: %v", perr)
			}
			compareGolden(t, msg, goldenRoot, fname)
		})
	}
}

// TestInspector2_NewFromConfig is a smoke test for the production ctor.
func TestInspector2_NewFromConfig(t *testing.T) {
	p := NewFromConfig(aws.Config{}, "", 0)
	if p == nil {
		t.Fatal("nil parser")
	}
	// pass a non-empty table name to exercise the wiring branch.
	p2 := NewFromConfig(aws.Config{}, "some-table", 14)
	if p2 == nil || p2.dedup == nil {
		t.Fatal("expected dedup wiring")
	}
}

func readEvent(t *testing.T, path string) *envelope.Event {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	ev, err := envelope.New(raw)
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	return ev
}

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
