package codebuild

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
	samplesRoot = "../../../samples/codebuild"
	goldenRoot  = "testdata/golden"
)

func TestCodeBuild_Name(t *testing.T) {
	if got := New().Name(); got != "codebuild" {
		t.Fatalf("Name = %q, want %q", got, "codebuild")
	}
}

func TestCodeBuild_Match(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "wrong-source", raw: `{"source":"aws.codepipeline","detail-type":"x"}`, want: false},
		{name: "right-source", raw: `{"source":"aws.codebuild","detail-type":"x"}`, want: true},
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

func TestCodeBuild_Parse_PhaseChangeSilenced(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(samplesRoot, "build_phase_change.json")) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("read sample: %v", err)
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
		t.Fatal("Build Phase Change must produce a silenced (nil) message")
	}
}

func TestCodeBuild_Parse_ErrorOnMalformedDetail(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{"source":"aws.codebuild","detail-type":"CodeBuild Build State Change","detail":{}}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := New().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error when detail is missing project-name")
	}
}

func TestCodeBuild_Parse_ErrorOnUnknownDetailType(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`{"source":"aws.codebuild","detail-type":"Bogus","detail":{"project-name":"p"}}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := New().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error on unexpected detail-type")
	}
}

func TestCodeBuild_SampleGoldens(t *testing.T) {
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
				t.Fatal("Match should be true for codebuild sample")
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
