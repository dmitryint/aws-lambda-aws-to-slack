package ses

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
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser"
)

var updateGoldens = flag.Bool("update", false, "rewrite golden files instead of comparing")

// runSESGoldenSuite is the shared driver for the three SES golden suites
// (bounce / complaint / received). It loads every .json fixture under
// samplesRoot, asserts the parser matches it, parses it, and compares the
// rendered Slack message against the matching golden file. Run with
// -update to rewrite the goldens after a deliberate change.
func runSESGoldenSuite(t *testing.T, p parser.Parser, samplesRoot, goldenRoot string) {
	t.Helper()
	entries, err := os.ReadDir(samplesRoot)
	if err != nil {
		t.Fatalf("read samples %s: %v", samplesRoot, err)
	}
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
				t.Fatalf("Match should be true for %s sample", p.Name())
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

func TestSES_MatchNotification_RejectsNonObjects(t *testing.T) {
	cases := []string{`"x"`, `[]`, `123`}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			ev, err := envelope.New(json.RawMessage(raw))
			if err != nil {
				return
			}
			if matchNotification(ev, notifBounce) {
				t.Fatalf("matchNotification should return false for %q", raw)
			}
		})
	}
}
