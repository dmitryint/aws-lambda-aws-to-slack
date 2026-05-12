package inspector

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
	samplesRoot = "../../../samples/inspector/classic"
	goldenRoot  = "testdata/golden"
)

func TestInspector_Name(t *testing.T) {
	if got := New().Name(); got != "inspector" {
		t.Fatalf("Name = %q, want %q", got, "inspector")
	}
}

func TestInspector_Match(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{
			name: "right-template",
			raw:  buildSNS(`{"event":"ASSESSMENT_RUN_STARTED","template":"arn:aws:inspector:us-east-1:x:y","target":"t"}`),
			want: true,
		},
		{
			name: "wrong-template-prefix",
			raw:  buildSNS(`{"event":"X","template":"arn:aws:other-service:us-east-1:x:y"}`),
			want: false,
		},
		{
			name: "missing-template",
			raw:  buildSNS(`{"event":"X"}`),
			want: false,
		},
		{
			name: "not-json",
			raw:  buildSNS(`not-json`),
			want: false,
		},
	}
	p := New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev, err := envelope.New(json.RawMessage(tc.raw))
			if err != nil {
				t.Fatalf("envelope.New: %v", err)
			}
			rec := ev.Records()[0]
			if got := p.Match(rec); got != tc.want {
				t.Fatalf("Match = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestInspector_Parse_EnableNotificationsIsSilenced(t *testing.T) {
	raw := buildSNS(`{"event":"ENABLE_ASSESSMENT_NOTIFICATIONS","template":"arn:aws:inspector:us-east-1:x:y","target":"t"}`)
	ev, err := envelope.New(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	msg, err := New().Parse(context.Background(), ev.Records()[0])
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if msg != nil {
		t.Fatalf("ENABLE_ASSESSMENT_NOTIFICATIONS should be silenced (nil), got %+v", msg)
	}
}

func TestInspector_Parse_UnknownEventStillRenders(t *testing.T) {
	raw := buildSNS(`{"event":"WAT","template":"arn:aws:inspector:us-east-1:x:y","target":"t"}`)
	ev, err := envelope.New(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	msg, err := New().Parse(context.Background(), ev.Records()[0])
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if msg == nil {
		t.Fatal("Parse should still emit a message for unknown events")
	}
}

func TestInspector_StateChangeFallbackToRaw(t *testing.T) {
	if got := renderStateChange("WAT_STATE"); got != "WAT_STATE" {
		t.Fatalf("renderStateChange unknown = %q, want passthrough", got)
	}
	if got := renderStateChange("COMPLETED"); got != "Completed" {
		t.Fatalf("renderStateChange COMPLETED = %q, want Completed", got)
	}
}

func TestInspector_ParseFindingsCount_HandlesEmpty(t *testing.T) {
	if got := parseFindingsCount(""); got != nil {
		t.Fatalf("parseFindingsCount empty = %v, want nil", got)
	}
}

func TestInspector_FormatFinding_UnknownArnPassesThrough(t *testing.T) {
	got := formatFinding(" arn:aws:inspector:us-east-1:000000000000:rulespackage/0-XXXXX=7 ")
	want := "arn:aws:inspector:us-east-1:000000000000:rulespackage/0-XXXXX: 7"
	if got != want {
		t.Fatalf("formatFinding = %q, want %q", got, want)
	}
}

func TestInspector_FormatFinding_KnownArnMapsToReadableName(t *testing.T) {
	got := formatFinding("arn:aws:inspector:us-east-1:316112463485:rulespackage/0-gEjTy7T7=12")
	want := "Common Vulnerabilities and Exposures: 12"
	if got != want {
		t.Fatalf("formatFinding mapping = %q, want %q", got, want)
	}
}

func TestInspector_FormatFinding_MissingEqualsDefaultsToZero(t *testing.T) {
	got := formatFinding("arn:aws:inspector:us-east-1:000000000000:rulespackage/0-X")
	want := "arn:aws:inspector:us-east-1:000000000000:rulespackage/0-X: 0"
	if got != want {
		t.Fatalf("formatFinding no-eq = %q, want %q", got, want)
	}
}

func TestInspector_RunURL_InvalidArnFallsBackToInvalid(t *testing.T) {
	got := runURL("run", "not-an-inspector-arn")
	if !strings.Contains(got, "region=invalid") {
		t.Fatalf("runURL invalid arn = %q, want region=invalid", got)
	}
}

func TestInspector_SampleGoldens(t *testing.T) {
	entries, err := os.ReadDir(samplesRoot)
	if err != nil {
		t.Fatalf("read samples: %v", err)
	}
	p := New()
	for _, entry := range entries {
		fname := entry.Name()
		if !strings.HasSuffix(fname, ".json") {
			continue
		}
		t.Run(fname, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(samplesRoot, fname)) //nolint:gosec // test fixture path
			if err != nil {
				t.Fatalf("read sample: %v", err)
			}
			ev, err := envelope.New(raw)
			if err != nil {
				t.Fatalf("envelope.New: %v", err)
			}
			rec := ev.Records()[0]
			if !p.Match(rec) {
				t.Fatal("Match should be true for inspector classic sample")
			}
			msg, perr := p.Parse(context.Background(), rec)
			if perr != nil {
				t.Fatalf("Parse: %v", perr)
			}
			gotJSON, err := json.MarshalIndent(msg, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			goldenPath := filepath.Join(goldenRoot, fname)
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

// buildSNS wraps the given Inspector-style inner Message in a minimal SNS
// envelope so the envelope package recognizes it as an SNS record.
func buildSNS(innerMessage string) string {
	encoded, err := json.Marshal(innerMessage)
	if err != nil {
		panic(err)
	}
	return `{"Records":[{"EventSource":"aws:sns","Sns":{"Message":` + string(encoded) + `,"Timestamp":"2024-06-01T12:00:00.000Z","Subject":"Amazon Inspector notification"}}]}`
}
