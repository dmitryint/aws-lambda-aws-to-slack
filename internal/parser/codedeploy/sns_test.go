package codedeploy

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
	snsSamplesRoot = "../../../samples/codedeploy/sns"
	snsGoldenRoot  = "testdata/golden/sns"
)

func TestSNS_Name(t *testing.T) {
	if got := NewSNS().Name(); got != "codedeploy-sns" {
		t.Fatalf("Name = %q, want %q", got, "codedeploy-sns")
	}
}

func TestSNS_Match_RequiresBothFields(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "missing-both", raw: `{}`, want: false},
		{name: "missing-group", raw: `{"deploymentId":"d-1"}`, want: false},
		{name: "missing-id", raw: `{"deploymentGroupName":"g"}`, want: false},
		{name: "both-present", raw: `{"deploymentId":"d-1","deploymentGroupName":"g"}`, want: true},
		{name: "non-object", raw: `"hello"`, want: false},
	}
	p := NewSNS()
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

func TestSNS_Parse_ErrorWhenInvalid(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, perr := NewSNS().Parse(context.Background(), ev); perr == nil {
		t.Fatal("Parse should error when payload lacks required fields")
	}
}

func TestSNS_Parse_UnknownStatusFallsBackToNeutral(t *testing.T) {
	raw := json.RawMessage(`{"deploymentId":"d-1","deploymentGroupName":"g","applicationName":"a","status":"WEIRD"}`)
	ev, err := envelope.New(raw)
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	msg, perr := NewSNS().Parse(context.Background(), ev)
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

func TestSNS_SampleGoldens(t *testing.T) {
	entries, err := os.ReadDir(snsSamplesRoot)
	if err != nil {
		t.Fatalf("read samples: %v", err)
	}
	p := NewSNS()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(snsSamplesRoot, name)) //nolint:gosec // test fixture path
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
				t.Fatal("Match should be true for codedeploy SNS sample")
			}
			msg, perr := p.Parse(context.Background(), rec)
			if perr != nil {
				t.Fatalf("Parse: %v", perr)
			}
			gotJSON, err := json.MarshalIndent(msg, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			goldenPath := filepath.Join(snsGoldenRoot, strings.TrimSuffix(name, ".json")+".json")
			if *updateGoldens {
				if err := os.MkdirAll(snsGoldenRoot, 0o750); err != nil {
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
