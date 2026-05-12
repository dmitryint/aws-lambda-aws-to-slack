package guardduty

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

var updateGoldens = flag.Bool("update", false, "rewrite golden files instead of comparing")

const (
	samplesRoot = "../../../samples/guardduty"
	goldenRoot  = "testdata/golden"
)

func TestGuardDuty_Name(t *testing.T) {
	if got := New().Name(); got != "guardduty" {
		t.Fatalf("Name = %q, want %q", got, "guardduty")
	}
}

func TestGuardDuty_Match(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "eventbridge-source", raw: `{"source":"aws.guardduty"}`, want: true},
		{name: "sns-service-name", raw: `{"detail":{"service":{"serviceName":"guardduty"}}}`, want: true},
		{name: "wrong-source-other-service", raw: `{"source":"aws.ec2","detail":{"service":{"serviceName":"other"}}}`, want: false},
		{name: "wrong-source-empty-detail", raw: `{"source":"aws.ec2"}`, want: false},
		{name: "malformed-detail", raw: `{"detail":"not-an-object"}`, want: false},
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

func TestGuardDuty_Parse_ErrorOnMissingDetail(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{"source":"aws.guardduty"}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := New().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error when detail is missing")
	}
}

func TestGuardDuty_Parse_ErrorOnMalformedDetail(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{"source":"aws.guardduty","detail":"not-an-object"}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := New().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error when detail is malformed")
	}
}

func TestGuardDuty_SeverityColor(t *testing.T) {
	cases := []struct {
		severity float64
		want     string
	}{
		{severity: 0, want: slack.ColorNeutral},
		{severity: 4, want: slack.ColorNeutral},
		{severity: 4.1, want: slack.ColorWarning},
		{severity: 7, want: slack.ColorWarning},
		{severity: 7.5, want: slack.ColorCritical},
		{severity: 9, want: slack.ColorCritical},
	}
	for _, tc := range cases {
		if got := severityColor(tc.severity); got != tc.want {
			t.Fatalf("severityColor(%g) = %q, want %q", tc.severity, got, tc.want)
		}
	}
}

func TestGuardDuty_FormatSeverity_WholeNumberHasNoDot(t *testing.T) {
	if got := formatSeverity(5); got != "5" {
		t.Fatalf("formatSeverity(5) = %q, want %q", got, "5")
	}
	if got := formatSeverity(4.5); got != "4.5" {
		t.Fatalf("formatSeverity(4.5) = %q, want %q", got, "4.5")
	}
}

func TestGuardDuty_PrettyJSON_FallsBackOnInvalid(t *testing.T) {
	if got := prettyJSON(nil); got != "" {
		t.Fatalf("prettyJSON(nil) = %q, want empty", got)
	}
	if got := prettyJSON(json.RawMessage(`not-json`)); got != "not-json" {
		t.Fatalf("prettyJSON(not-json) = %q, want passthrough", got)
	}
}

func TestGuardDuty_SampleGoldens(t *testing.T) {
	entries, err := os.ReadDir(samplesRoot)
	if err != nil {
		t.Fatalf("read samples: %v", err)
	}
	p := New()
	for _, entry := range entries {
		fname := entry.Name()
		if !strings.HasSuffix(fname, ".json") {
			continue
		}
		t.Run(fname, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(samplesRoot, fname)) //nolint:gosec // test fixture path
			if err != nil {
				t.Fatalf("read sample: %v", err)
			}
			ev, err := envelope.New(raw)
			if err != nil {
				t.Fatalf("envelope.New: %v", err)
			}
			rec := ev.Records()[0]
			if !p.Match(rec) {
				t.Fatal("Match should be true for guardduty sample")
			}
			msg, perr := p.Parse(context.Background(), rec)
			if perr != nil {
				t.Fatalf("Parse: %v", perr)
			}
			gotJSON, err := json.MarshalIndent(msg, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			goldenPath := filepath.Join(goldenRoot, fname)
			if *updateGoldens {
				if err := os.MkdirAll(goldenRoot, 0o750); err != nil {
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
		})
	}
}
