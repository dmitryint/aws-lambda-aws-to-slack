package beanstalk

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
	samplesRoot = "../../../samples/beanstalk"
	goldenRoot  = "testdata/golden"
)

func TestBeanstalk_Name(t *testing.T) {
	if got := New().Name(); got != "beanstalk" {
		t.Fatalf("Name = %q, want %q", got, "beanstalk")
	}
}

func TestBeanstalk_Match_OnlyOnPrefix(t *testing.T) {
	cases := []struct {
		name    string
		subject string
		want    bool
	}{
		{name: "empty-subject", subject: "", want: false},
		{name: "unrelated", subject: "Auto Scaling: launch", want: false},
		{name: "exact-prefix", subject: subjectPrefix, want: true},
		{name: "prefix-plus-suffix", subject: subjectPrefix + " - foo", want: true},
	}
	p := New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{
				"Records": []any{map[string]any{
					"EventSource":          "aws:sns",
					"EventSubscriptionArn": "arn:aws:sns:us-east-1:1:t:s",
					"Sns": map[string]any{
						"Message": "Application: a\nEnvironment: e\nMessage: ok\nTimestamp: 2024",
						"Subject": tc.subject,
					},
				}},
			})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			ev, err := envelope.New(raw)
			if err != nil {
				t.Fatalf("envelope.New: %v", err)
			}
			if got := p.Match(ev); got != tc.want {
				t.Fatalf("Match = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBeanstalk_Parse_SilencesUnparseableBody(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"Records": []any{map[string]any{
			"EventSource": "aws:sns",
			"Sns": map[string]any{
				"Message": "this body has no recognizable fields\n",
				"Subject": subjectPrefix + " - whatever",
			},
		}},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ev, err := envelope.New(raw)
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	msg, perr := New().Parse(context.Background(), ev)
	if perr != nil {
		t.Fatalf("Parse: %v", perr)
	}
	if msg != nil {
		t.Fatalf("expected silenced parse (nil), got %+v", msg)
	}
}

func TestClassify_SeverityTable(t *testing.T) {
	cases := []struct {
		name string
		text string
		want notify.Severity
	}{
		{name: "info", text: "Environment health has transitioned from YELLOW to Ok", want: notify.SeverityInfo},
		{name: "red-critical", text: "Environment health has transitioned from Ok to RED", want: notify.SeverityCritical},
		{name: "yellow-warning", text: "Environment health has transitioned from Ok to YELLOW", want: notify.SeverityWarning},
		{name: "deploy-failed", text: "Failed to deploy application.", want: notify.SeverityCritical},
		{name: "aborted-operation", text: "The environment update aborted operation. ok", want: notify.SeverityWarning},
		{name: "removed-instance", text: "Removed instance i-12345 from the environment", want: notify.SeverityWarning},
		// When both lists match, the second `if` block wins — warning overrides critical.
		{name: "warning-overrides-critical", text: "transitioned to RED and to YELLOW", want: notify.SeverityWarning},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classify(tc.text); got != tc.want {
				t.Fatalf("classify = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestBeanstalk_SampleGoldens(t *testing.T) {
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
				t.Fatal("Match should be true for beanstalk sample")
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
