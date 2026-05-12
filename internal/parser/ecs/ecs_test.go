package ecs

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
	samplesRoot = "../../../samples/ecs"
	goldenRoot  = "testdata/golden"
)

func TestECS_Name(t *testing.T) {
	if got := New().Name(); got != "ecs" {
		t.Fatalf("Name = %q, want %q", got, "ecs")
	}
}

func TestECS_Match(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "wrong-source", raw: `{"source":"aws.codepipeline"}`, want: false},
		{name: "right-source", raw: `{"source":"aws.ecs"}`, want: true},
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

func TestECS_TaskColor(t *testing.T) {
	cases := []struct {
		last, desired, want string
	}{
		{last: "RUNNING", desired: "RUNNING", want: "good"},
		{last: "PENDING", desired: "RUNNING", want: "warning"},
		{last: "STOPPED", desired: "STOPPED", want: "danger"},
		{last: "RUNNING", desired: "STOPPED", want: "danger"},
	}
	for _, tc := range cases {
		if got := taskColor(tc.last, tc.desired); got != tc.want {
			t.Fatalf("taskColor(%q,%q) = %q, want %q", tc.last, tc.desired, got, tc.want)
		}
	}
}

func TestECS_ServiceColor(t *testing.T) {
	cases := []struct {
		eventType, want string
	}{
		{eventType: "INFO", want: "good"},
		{eventType: "WARN", want: "warning"},
		{eventType: "ERROR", want: "danger"},
		{eventType: "OTHER", want: "#dddddd"},
	}
	for _, tc := range cases {
		if got := serviceColor(tc.eventType); got != tc.want {
			t.Fatalf("serviceColor(%q) = %q, want %q", tc.eventType, got, tc.want)
		}
	}
}

func TestECS_Parse_ErrorOnMissingTaskDetail(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{"source":"aws.ecs","detail-type":"ECS Task State Change"}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := New().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error when task detail is missing")
	}
}

func TestECS_Parse_ErrorOnMissingServiceDetail(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{"source":"aws.ecs","detail-type":"ECS Service Action"}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := New().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error when service detail is missing")
	}
}

func TestECS_SampleGoldens(t *testing.T) {
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
				t.Fatal("Match should be true for ecs sample")
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
