package codedeploy

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

const (
	cloudWatchSamplesRoot = "../../../samples/codedeploy/eventbridge"
	cloudWatchGoldenRoot  = "testdata/golden/cloudwatch"
)

func TestCloudWatch_Name(t *testing.T) {
	if got := NewCloudWatch().Name(); got != "codedeploy-cloudwatch" {
		t.Fatalf("Name = %q, want %q", got, "codedeploy-cloudwatch")
	}
}

func TestCloudWatch_Match(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "wrong-source", raw: `{"source":"aws.codepipeline"}`, want: false},
		{name: "right-source", raw: `{"source":"aws.codedeploy"}`, want: true},
		{name: "empty", raw: `{}`, want: false},
	}
	p := NewCloudWatch()
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

func TestCloudWatch_Parse_ErrorWhenInvalid(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{"source":"aws.codedeploy"}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := NewCloudWatch().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error when detail block missing")
	}
}

func TestCloudWatch_Parse_UnknownStateFallsBackToNeutral(t *testing.T) {
	raw := json.RawMessage(`{"source":"aws.codedeploy","region":"us-east-1","detail":{"state":"WEIRD","deploymentGroup":"g","deploymentId":"d","application":"a"}}`)
	ev, err := envelope.New(raw)
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	msg, perr := NewCloudWatch().Parse(context.Background(), ev)
	if perr != nil {
		t.Fatalf("Parse: %v", perr)
	}
	if msg == nil {
		t.Fatal("expected message")
	}
	if msg.Severity != notify.SeverityNotice {
		t.Fatalf("severity = %s, want %s", msg.Severity, notify.SeverityNotice)
	}
}

func TestCloudWatch_SampleGoldens(t *testing.T) {
	entries, err := os.ReadDir(cloudWatchSamplesRoot)
	if err != nil {
		t.Fatalf("read samples: %v", err)
	}
	p := NewCloudWatch()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(cloudWatchSamplesRoot, name)) //nolint:gosec // test fixture path
			if err != nil {
				t.Fatalf("read sample: %v", err)
			}
			ev, err := envelope.New(raw)
			if err != nil {
				t.Fatalf("envelope.New: %v", err)
			}
			rec := ev.Records()[0]
			if !p.Match(rec) {
				t.Fatal("Match should be true for codedeploy eventbridge sample")
			}
			msg, perr := p.Parse(context.Background(), rec)
			if perr != nil {
				t.Fatalf("Parse: %v", perr)
			}
			gotJSON, err := json.MarshalIndent(msg, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			goldenPath := filepath.Join(cloudWatchGoldenRoot, strings.TrimSuffix(name, ".json")+".json")
			if *updateGoldens {
				if err := os.MkdirAll(cloudWatchGoldenRoot, 0o750); err != nil {
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
