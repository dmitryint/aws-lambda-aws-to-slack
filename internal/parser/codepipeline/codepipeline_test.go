package codepipeline

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
	pipelineSamplesRoot = "../../../samples/codepipeline"
	approvalSamplesRoot = "../../../samples/codepipeline/approval"
	pipelineGoldenRoot  = "testdata/golden/pipeline"
	approvalGoldenRoot  = "testdata/golden/approval"
)

func TestPipeline_Name(t *testing.T) {
	if got := New().Name(); got != "codepipeline" {
		t.Fatalf("Name = %q, want %q", got, "codepipeline")
	}
}

func TestApproval_Name(t *testing.T) {
	if got := NewApproval().Name(); got != "codepipeline-approval" {
		t.Fatalf("Name = %q, want %q", got, "codepipeline-approval")
	}
}

// TestPipeline_Match_RejectsApprovalPayload verifies the pipeline parser
// explicitly rejects payloads that carry an approval block, leaving them to
// the approval parser regardless of router order.
func TestPipeline_Match_RejectsApprovalPayload(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "no-source", raw: `{}`, want: false},
		{name: "wrong-source", raw: `{"source":"aws.ecs"}`, want: false},
		{name: "right-source-no-approval", raw: `{"source":"aws.codepipeline"}`, want: true},
		{name: "right-source-with-approval", raw: `{"source":"aws.codepipeline","approval":{"pipelineName":"p"}}`, want: false},
		{name: "right-source-approval-empty-name", raw: `{"source":"aws.codepipeline","approval":{}}`, want: true},
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

func TestPipeline_SampleGoldens(t *testing.T) {
	entries, err := os.ReadDir(pipelineSamplesRoot)
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
			raw, err := os.ReadFile(filepath.Join(pipelineSamplesRoot, name)) //nolint:gosec // test fixture path
			if err != nil {
				t.Fatalf("read sample: %v", err)
			}
			ev, err := envelope.New(raw)
			if err != nil {
				t.Fatalf("envelope.New: %v", err)
			}
			rec := ev.Records()[0]
			if !p.Match(rec) {
				t.Fatal("Match should be true for codepipeline sample")
			}
			msg, perr := p.Parse(context.Background(), rec)
			if perr != nil {
				t.Fatalf("Parse: %v", perr)
			}
			compareGolden(t, msg, pipelineGoldenRoot, name)
		})
	}
}

func TestApproval_SampleGoldens(t *testing.T) {
	entries, err := os.ReadDir(approvalSamplesRoot)
	if err != nil {
		t.Fatalf("read samples: %v", err)
	}
	p := NewApproval()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(approvalSamplesRoot, name)) //nolint:gosec // test fixture path
			if err != nil {
				t.Fatalf("read sample: %v", err)
			}
			ev, err := envelope.New(raw)
			if err != nil {
				t.Fatalf("envelope.New: %v", err)
			}
			rec := ev.Records()[0]
			if !p.Match(rec) {
				t.Fatal("Match should be true for codepipeline approval sample")
			}
			msg, perr := p.Parse(context.Background(), rec)
			if perr != nil {
				t.Fatalf("Parse: %v", perr)
			}
			compareGolden(t, msg, approvalGoldenRoot, name)
		})
	}
}

func TestApproval_Match_RequiresBothFields(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "missing-both", raw: `{}`, want: false},
		{name: "missing-approval", raw: `{"consoleLink":"x"}`, want: false},
		{name: "missing-console", raw: `{"approval":{"pipelineName":"p"}}`, want: false},
		{name: "both-present", raw: `{"consoleLink":"x","approval":{"pipelineName":"p"}}`, want: true},
	}
	p := NewApproval()
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

func TestApproval_Parse_ErrorWhenInvalid(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := NewApproval().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error when payload lacks required fields")
	}
}

func TestApproval_RenderHours(t *testing.T) {
	cases := []struct {
		numHours float64
		want     string
	}{
		{numHours: -1, want: "*-1 ago!*"},
		{numHours: -0.0167, want: "*0 ago!*"},
		{numHours: 0.5, want: "within *30 minutes*"},
		{numHours: 5, want: "within 5 hours"},
		{numHours: 120, want: "within 5 days"},
	}
	for _, tc := range cases {
		if got := renderHours(tc.numHours); got != tc.want {
			t.Fatalf("renderHours(%g) = %q, want %q", tc.numHours, got, tc.want)
		}
	}
}

// compareGolden serializes msg to JSON, then either writes it as the new
// golden (when -update is set) or compares it against the existing one.
func compareGolden(t *testing.T, msg any, dir, sampleName string) {
	t.Helper()
	gotJSON, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	goldenPath := filepath.Join(dir, strings.TrimSuffix(sampleName, ".json")+".json")
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
