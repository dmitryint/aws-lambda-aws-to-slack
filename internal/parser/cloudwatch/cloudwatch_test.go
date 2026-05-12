package cloudwatch

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/go-cmp/cmp"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
)

var updateGoldens = flag.Bool("update", false, "rewrite golden files instead of comparing")

const (
	samplesRoot = "../../../samples/cloudwatch"
	goldenRoot  = "testdata/golden"
)

func TestCloudwatch_Name(t *testing.T) {
	if got := New().Name(); got != "cloudwatch" {
		t.Fatalf("Name = %q", got)
	}
}

func TestCloudwatch_Match(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{
			name: "alarm-payload",
			raw:  buildSNS(`{"AlarmName":"x","AlarmDescription":"y"}`),
			want: true,
		},
		{
			name: "missing-description",
			raw:  buildSNS(`{"AlarmName":"x"}`),
			want: false,
		},
		{
			name: "missing-name",
			raw:  buildSNS(`{"AlarmDescription":"y"}`),
			want: false,
		},
		{
			name: "empty-object",
			raw:  buildSNS(`{}`),
			want: false,
		},
		{
			name: "plain-text",
			raw:  buildSNS(`not-json`),
			want: false,
		},
	}
	p := New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev, err := envelope.New(json.RawMessage(tc.raw))
			if err != nil {
				t.Fatalf("envelope.New: %v", err)
			}
			rec := ev.Records()[0]
			if got := p.Match(rec); got != tc.want {
				t.Fatalf("Match = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCloudwatch_StateColor(t *testing.T) {
	cases := map[string]string{
		"OK":                "good",
		"ALARM":             "danger",
		"INSUFFICIENT_DATA": "warning",
		"OTHER":             "#dddddd",
	}
	for state, want := range cases {
		if got := stateColor(state); got != want {
			t.Fatalf("stateColor(%s) = %q, want %q", state, got, want)
		}
	}
}

func TestCloudwatch_ResolveRegion(t *testing.T) {
	if got := resolveRegion("US East (N. Virginia)", "fallback"); got != "us-east-1" {
		t.Fatalf("got %q", got)
	}
	if got := resolveRegion("Mars", "us-west-2"); got != "us-west-2" {
		t.Fatalf("got %q", got)
	}
}

func TestCloudwatch_DecodeAlarm_RejectsEmptyName(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(buildSNS(`{"AlarmDescription":"x"}`)))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, ok := decodeAlarm(ev.Records()[0]); ok {
		t.Fatal("decode should reject payload without AlarmName")
	}
}

func TestCloudwatch_PresignerAdapter_WrapsRealSDK(t *testing.T) {
	// build a real presigner from an offline config; PresignGetObject only
	// signs locally (no network) so it succeeds without credentials when
	// they're set on the config.
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: staticCredsForTest{},
	}
	client := s3.NewFromConfig(cfg)
	a := presignerAdapter{presigner: s3.NewPresignClient(client)}
	req, err := a.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("k"),
	})
	if err != nil {
		t.Fatalf("PresignGetObject: %v", err)
	}
	if req == nil || req.URL == "" || req.Method == "" {
		t.Fatalf("got %+v", req)
	}
}

// staticCredsForTest is a no-network CredentialsProvider used only to give
// the local presigner something to sign with.
type staticCredsForTest struct{}

func (staticCredsForTest) Retrieve(_ context.Context) (aws.Credentials, error) {
	return aws.Credentials{AccessKeyID: "AKIA", SecretAccessKey: "SECRET"}, nil
}

func TestCloudwatch_Parse_ErrorOnBadPayload(t *testing.T) {
	raw := buildSNS(`not-json`)
	ev, err := envelope.New(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := New().Parse(context.Background(), ev.Records()[0]); err == nil {
		t.Fatal("Parse should error on non-JSON payload")
	}
}

func TestCloudwatch_SampleGoldens(t *testing.T) {
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

// TestCloudwatch_LambdaAlarm_RendersLogsLink verifies that a Lambda Errors
// alarm with a FunctionName dimension gets a "See recent logs" link in
// the body.
func TestCloudwatch_LambdaAlarm_RendersLogsLink(t *testing.T) {
	ev := readEvent(t, filepath.Join(samplesRoot, "alarm_lambda_with_log_link.json"))
	msg, err := New().Parse(context.Background(), ev)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !messageContains(msg, "See recent logs") {
		t.Fatalf("expected See recent logs link, got %+v", msg)
	}
	if !messageContains(msg, "%2Faws%2Flambda%2Faws-to-slack") {
		t.Fatalf("expected encoded /aws/lambda/aws-to-slack in URL")
	}
}

// TestCloudwatch_NewFromConfig_EmptyBucket_NoChart asserts that the
// production ctor returns a no-chart parser when CHART_BUCKET_NAME is unset.
func TestCloudwatch_NewFromConfig_EmptyBucket_NoChart(t *testing.T) {
	p := NewFromConfig(aws.Config{}, ChartConfig{})
	if p == nil {
		t.Fatal("nil parser")
	}
	if p.pipeline != nil {
		t.Fatal("expected nil pipeline when bucket name is empty")
	}
}

// TestCloudwatch_NewFromConfig_WithBucket_HasPipeline asserts that the
// production ctor wires the real pipeline when CHART_BUCKET_NAME is set.
// We do NOT exercise the pipeline (no SDK calls in tests).
func TestCloudwatch_NewFromConfig_WithBucket_HasPipeline(t *testing.T) {
	p := NewFromConfig(aws.Config{Region: "us-east-1"}, ChartConfig{
		BucketName:     "test-bucket",
		BucketRegion:   "us-west-2",
		FallbackRegion: "us-east-1",
	})
	if p == nil || p.pipeline == nil {
		t.Fatal("expected pipeline when bucket is configured")
	}
}

// TestCloudwatch_AlarmConsoleURL covers the per-partition console host
// branches.
func TestCloudwatch_AlarmConsoleURL(t *testing.T) {
	cases := []struct {
		region   string
		contains string
	}{
		{"us-east-1", "https://console.aws.amazon.com"},
		{"cn-north-1", "https://console.amazonaws.cn"},
		{"us-gov-west-1", "https://console.amazonaws-us-gov.com"},
	}
	for _, tc := range cases {
		got := alarmConsoleURL(tc.region, "x")
		if !strings.HasPrefix(got, tc.contains) {
			t.Fatalf("alarmConsoleURL(%s) = %q, want prefix %q", tc.region, got, tc.contains)
		}
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
	return ev.Records()[0]
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

func messageContains(msg any, sub string) bool {
	b, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	return strings.Contains(string(b), sub)
}

// buildSNS wraps the given alarm-style inner Message in an SNS envelope.
func buildSNS(inner string) string {
	encoded, err := json.Marshal(inner)
	if err != nil {
		panic(err)
	}
	return `{"Records":[{"EventSource":"aws:sns","Sns":{"Message":` + string(encoded) +
		`,"Timestamp":"2024-06-01T12:00:00.000Z","Subject":"ALARM"}}]}`
}
