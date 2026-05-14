package batch

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
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

var updateGoldens = flag.Bool("update", false, "rewrite golden files instead of comparing")

const (
	samplesRoot = "../../../samples/batch"
	goldenRoot  = "testdata/golden"
)

func TestBatch_Name(t *testing.T) {
	if got := New().Name(); got != "batch" {
		t.Fatalf("Name = %q, want %q", got, "batch")
	}
}

func TestBatch_Match(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "right-source", raw: `{"source":"aws.batch"}`, want: true},
		{name: "wrong-source", raw: `{"source":"aws.codebuild"}`, want: false},
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

func TestBatch_Parse_ErrorOnMissingDetail(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{"source":"aws.batch"}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := New().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error when detail is missing")
	}
}

func TestBatch_Parse_ErrorOnMissingJobName(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{"source":"aws.batch","detail":{"status":"SUCCEEDED"}}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := New().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error when jobName is missing")
	}
}

func TestBatch_SeverityAndTitle(t *testing.T) {
	cases := []struct {
		status string
		want   notify.Severity
	}{
		{"SUBMITTED", notify.SeverityNotice},
		{"RUNNABLE", notify.SeverityNotice},
		{"STARTING", notify.SeverityNotice},
		{"RUNNING", notify.SeverityNotice},
		{"SUCCEEDED", notify.SeverityOK},
		{"FAILED", notify.SeverityCritical},
		{"WEIRD", notify.SeverityNotice},
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			got, _ := severityAndTitle("job", tc.status)
			if got != tc.want {
				t.Fatalf("severity = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestBatch_SampleGoldens(t *testing.T) {
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
				t.Fatal("Match should be true for batch sample")
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
