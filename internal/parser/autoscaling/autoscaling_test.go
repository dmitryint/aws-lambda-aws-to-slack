package autoscaling

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
	samplesRoot = "../../../samples/autoscaling"
	goldenRoot  = "testdata/golden"
)

func TestAutoScaling_Name(t *testing.T) {
	if got := New().Name(); got != "autoscaling" {
		t.Fatalf("Name = %q, want %q", got, "autoscaling")
	}
}

func TestAutoScaling_Match_RejectsUnrelated(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{name: "empty-object", raw: `{}`},
		{name: "string-message", raw: `"hello"`},
		{name: "no-arn", raw: `{"AutoScalingGroupName":"x"}`},
	}
	p := New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev, err := envelope.New(json.RawMessage(tc.raw))
			if err != nil {
				t.Fatalf("envelope.New: %v", err)
			}
			if p.Match(ev) {
				t.Fatal("Match should reject payloads without AutoScalingGroupARN")
			}
		})
	}
}

func TestAutoScaling_Parse_ErrorWhenMatchSkipped(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := New().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error when payload is malformed")
	}
}

func TestRegionFromARN_Bounds(t *testing.T) {
	if got := regionFromARN(""); got != "" {
		t.Fatalf("regionFromARN(empty) = %q, want empty", got)
	}
	if got := regionFromARN("arn:aws:autoscaling:eu-west-1:1:a:b"); got != "eu-west-1" {
		t.Fatalf("regionFromARN = %q", got)
	}
}

func TestAutoScaling_SampleGoldens(t *testing.T) {
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
			records := ev.Records()
			if len(records) == 0 {
				t.Fatal("no records produced")
			}
			rec := records[0]
			if !p.Match(rec) {
				t.Fatal("Match should be true for autoscaling sample")
			}
			msg, err := p.Parse(context.Background(), rec)
			if err != nil {
				t.Fatalf("Parse: %v", err)
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
