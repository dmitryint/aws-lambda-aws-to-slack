package generic

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
	samplesRoot = "../../../samples/generic"
	goldenRoot  = "testdata/golden"
)

func TestGeneric_AlwaysMatches(t *testing.T) {
	p := New()
	ev, err := envelope.New(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if !p.Match(ev) {
		t.Fatal("generic parser must always match")
	}
}

func TestGeneric_Name(t *testing.T) {
	if got := New().Name(); got != "generic" {
		t.Fatalf("Name = %q, want %q", got, "generic")
	}
}

func TestGeneric_SampleGoldens(t *testing.T) {
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
			msg, err := p.Parse(context.Background(), records[0])
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

func TestGeneric_StringMessage(t *testing.T) {
	// Plain SNS payload — Sns.Message is a free-form string.
	raw := json.RawMessage(`{"Records":[{"EventSource":"aws:sns","EventSubscriptionArn":"arn:aws:sns:us-east-1:123:t","Sns":{"Message":"hello world","Subject":"Custom","Timestamp":"2024-06-01T12:00:00Z"}}]}`)
	ev, err := envelope.New(raw)
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	msg, err := New().Parse(context.Background(), ev)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if msg == nil {
		t.Fatal("expected non-nil notification")
	}
	if msg.Title != "Custom" {
		t.Fatalf("title = %q, want %q", msg.Title, "Custom")
	}
	if !strings.Contains(msg.Summary, "hello world") {
		t.Fatalf("summary should contain string body, got %q", msg.Summary)
	}
}

func TestGeneric_NoSubjectFallsBackToRawEvent(t *testing.T) {
	// EventBridge envelope but pretend the source is missing entirely.
	raw := json.RawMessage(`{"detail":{"foo":"bar"}}`)
	ev, err := envelope.New(raw)
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	msg, err := New().Parse(context.Background(), ev)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message")
	}
}

func TestGeneric_LargeObjectFallsBackToCodeBlock(t *testing.T) {
	obj := map[string]any{}
	for i := 0; i < 12; i++ {
		obj[ascii(i)] = "value"
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ev, err := envelope.New(raw)
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	msg, err := New().Parse(context.Background(), ev)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message")
	}
	// When key count > maxFields, the renderer routes the payload into a
	// pretty-printed code block in Summary rather than per-field rows.
	if len(msg.Fields) != 0 {
		t.Fatalf("expected code-block path, got %d fields", len(msg.Fields))
	}
	if !strings.HasPrefix(msg.Summary, "```") {
		t.Fatalf("expected summary to start with code fence, got %q", msg.Summary)
	}
}

func TestGeneric_VersionEmptyValueSkipped(t *testing.T) {
	// `version` with an empty value should be elided.
	raw := json.RawMessage(`{"version":"", "alpha":"a", "beta":"b"}`)
	ev, err := envelope.New(raw)
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	msg, err := New().Parse(context.Background(), ev)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for _, f := range msg.Fields {
		if f.Key == "version" {
			t.Fatalf("version with empty value should be skipped: %+v", f)
		}
	}
}

func TestStringifyValue(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{name: "string", in: "abc", want: "abc"},
		{name: "single-element-string-array", in: []any{"alpha"}, want: "alpha"},
		{name: "multi-element-array", in: []any{"a", "b"}, want: `["a","b"]`},
		{name: "number", in: float64(42), want: "42"},
		{name: "object", in: map[string]any{"k": "v"}, want: `{"k":"v"}`},
		{name: "bool", in: true, want: "true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stringifyValue(tc.in); got != tc.want {
				t.Fatalf("stringifyValue(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func ascii(i int) string {
	return string(rune('a' + i))
}
