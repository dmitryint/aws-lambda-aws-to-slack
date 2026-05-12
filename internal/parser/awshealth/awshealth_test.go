package awshealth

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
)

var updateGoldens = flag.Bool("update", false, "rewrite golden files instead of comparing")

const (
	samplesRoot = "../../../samples/awshealth"
	goldenRoot  = "testdata/golden"
)

func TestAWSHealth_Name(t *testing.T) {
	if got := New().Name(); got != "awshealth" {
		t.Fatalf("Name = %q, want %q", got, "awshealth")
	}
}

func TestAWSHealth_Match(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "right-source", raw: `{"source":"aws.health"}`, want: true},
		{name: "wrong-source", raw: `{"source":"aws.ec2"}`, want: false},
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

func TestAWSHealth_Parse_ErrorOnMissingDetail(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{"source":"aws.health","detail-type":"AWS Health Event"}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := New().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error when detail is missing")
	}
}

func TestAWSHealth_Parse_FallsBackToFirstDescription(t *testing.T) {
	raw := `{
		"source":"aws.health",
		"detail-type":"AWS Health Event",
		"account":"123",
		"detail":{
			"eventTypeCategory":"accountNotification",
			"eventDescription":[{"language":"ja_JP","latestDescription":"hello"}]
		}
	}`
	ev, err := envelope.New(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	msg, err := New().Parse(context.Background(), ev)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if msg == nil || msg.Attachments[0].Fallback != "hello" {
		t.Fatalf("expected fallback 'hello', got %+v", msg)
	}
}

func TestAWSHealth_Parse_FallbackUsesDetailTypeWhenNoDescription(t *testing.T) {
	raw := `{
		"source":"aws.health",
		"detail-type":"AWS Health Event",
		"account":"123",
		"detail":{"eventTypeCategory":"accountNotification","eventDescription":[]}
	}`
	ev, err := envelope.New(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	msg, err := New().Parse(context.Background(), ev)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := msg.Attachments[0].Fallback; got != "AWS Health Event" {
		t.Fatalf("fallback = %q, want %q", got, "AWS Health Event")
	}
}

func TestAWSHealth_FormatMrkdwn_ReplacesSentinel(t *testing.T) {
	got := formatMrkdwn("line1//nline2//Nline3")
	want := "line1\nline2\nline3"
	if got != want {
		t.Fatalf("formatMrkdwn = %q, want %q", got, want)
	}
	if formatMrkdwn("") != "" {
		t.Fatal("formatMrkdwn empty input should return empty")
	}
}

func TestAWSHealth_FormatHealthTime_FallsBackOnUnparseable(t *testing.T) {
	got := formatHealthTime("not-a-date")
	if got != "not-a-date" {
		t.Fatalf("formatHealthTime unparseable = %q, want %q", got, "not-a-date")
	}
}

func TestAWSHealth_SampleGoldens(t *testing.T) {
	entries, err := os.ReadDir(samplesRoot)
	if err != nil {
		t.Fatalf("read samples: %v", err)
	}
	p := New()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(samplesRoot, name)) //nolint:gosec // test fixture path
			if err != nil {
				t.Fatalf("read sample: %v", err)
			}
			ev, err := envelope.New(raw)
			if err != nil {
				t.Fatalf("envelope.New: %v", err)
			}
			rec := ev.Records()[0]
			if !p.Match(rec) {
				t.Fatal("Match should be true for awshealth sample")
			}
			msg, perr := p.Parse(context.Background(), rec)
			if perr != nil {
				t.Fatalf("Parse: %v", perr)
			}
			gotJSON, err := json.MarshalIndent(msg, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			goldenPath := filepath.Join(goldenRoot, strings.TrimSuffix(name, ".json")+".json")
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
