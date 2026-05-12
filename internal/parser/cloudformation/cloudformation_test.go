package cloudformation

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
	samplesRoot = "../../../samples/cloudformation"
	goldenRoot  = "testdata/golden"
)

// silencedSamples lists the fixtures the parser is expected to drop via
// (nil, nil): the resource-level event (LogicalResourceId != StackName) and
// the malformed body. Their goldens hold the literal string "null\n".
var silencedSamples = map[string]bool{
	"resource_event_ignored.json": true,
	"malformed.json":              true,
}

func TestCloudFormation_Name(t *testing.T) {
	if got := New().Name(); got != "cloudformation" {
		t.Fatalf("Name = %q, want %q", got, "cloudformation")
	}
}

func TestCloudFormation_Match_OnlyOnPrefix(t *testing.T) {
	cases := []struct {
		name    string
		subject string
		want    bool
	}{
		{name: "empty", subject: "", want: false},
		{name: "wrong-prefix", subject: "Some other notification", want: false},
		{name: "exact-prefix", subject: subjectPrefix, want: true},
		{name: "with-suffix", subject: subjectPrefix + " - misc", want: true},
	}
	p := New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{
				"Records": []any{map[string]any{
					"EventSource": "aws:sns",
					"Sns": map[string]any{
						"Message": "",
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

func TestCloudFormation_SilencedPaths(t *testing.T) {
	cases := []struct {
		name    string
		message string
	}{
		{name: "missing-fields", message: "this body does not parse\n"},
		{
			name: "resource-event",
			message: "LogicalResourceId='MyLogGroup'\n" +
				"StackName='example-stack'\n" +
				"ResourceStatus='CREATE_COMPLETE'\n",
		},
	}
	p := New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{
				"Records": []any{map[string]any{
					"EventSource": "aws:sns",
					"Sns": map[string]any{
						"Message": tc.message,
						"Subject": subjectPrefix,
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
			msg, perr := p.Parse(context.Background(), ev)
			if perr != nil {
				t.Fatalf("Parse: %v", perr)
			}
			if msg != nil {
				t.Fatalf("expected silenced parse, got %+v", msg)
			}
		})
	}
}

func TestParseQuotedKeyValueLines(t *testing.T) {
	body := "Key1='value-one'\nKey2='value with spaces'\nbroken-line\nKey3=no-quotes\n"
	got := parseQuotedKeyValueLines(body)
	want := map[string]string{
		"Key1": "value-one",
		"Key2": "value with spaces",
		"Key3": "no-quotes",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("parseQuotedKeyValueLines (-want +got):\n%s", diff)
	}
}

func TestRegionFromARN_Empty(t *testing.T) {
	if got := regionFromARN(""); got != "" {
		t.Fatalf("regionFromARN(empty) = %q", got)
	}
	if got := regionFromARN("arn:aws:cloudformation:us-west-2:1:stack/x/y"); got != "us-west-2" {
		t.Fatalf("regionFromARN = %q", got)
	}
}

func TestCloudFormation_SampleGoldens(t *testing.T) {
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
				t.Fatal("Match should be true for any CloudFormation sample (subject-based)")
			}
			msg, perr := p.Parse(context.Background(), rec)
			if perr != nil {
				t.Fatalf("Parse: %v", perr)
			}

			var gotJSON []byte
			if silencedSamples[name] {
				if msg != nil {
					t.Fatalf("expected silenced parse for %s, got %+v", name, msg)
				}
				gotJSON = []byte("null")
			} else {
				if msg == nil {
					t.Fatalf("unexpected silence for %s", name)
				}
				gotJSON, err = json.MarshalIndent(msg, "", "  ")
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
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
