package slack

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

func TestRenderer_TextWithinBlockKitLimits(t *testing.T) {
	const (
		textObjectLimit = 3000
		fieldTextLimit  = 2000
	)
	longASCII := strings.Repeat("abcdefghij", 350)
	longMultibyte := strings.Repeat("é", 3500)
	cases := []struct {
		name      string
		n         notify.Notification
		preserved string
	}{
		{
			name:      "summary_over_section_limit",
			n:         notify.Notification{Severity: notify.SeverityNotice, Title: "T", Summary: longASCII},
			preserved: longASCII[:200],
		},
		{
			name:      "multibyte_summary_over_section_limit",
			n:         notify.Notification{Severity: notify.SeverityNotice, Title: "T", Summary: longMultibyte},
			preserved: strings.Repeat("é", 200),
		},
		{
			name:      "title_over_section_limit",
			n:         notify.Notification{Severity: notify.SeverityCritical, Title: longASCII, Subtitle: "ST"},
			preserved: longASCII[:200],
		},
		{
			name: "field_value_over_field_limit",
			n: notify.Notification{
				Severity: notify.SeverityWarning,
				Title:    "T",
				Fields:   []notify.Field{{Key: "Resource", Value: longASCII}},
			},
			preserved: longASCII[:200],
		},
		{
			name: "footnote_over_text_object_limit",
			n: notify.Notification{
				Severity:  notify.SeverityInfo,
				Title:     "T",
				Footnotes: []string{longASCII},
			},
			preserved: longASCII[:200],
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &recordingPoster{}
			n := tc.n
			if err := NewRenderer(p).Send(t.Context(), &n); err != nil {
				t.Fatalf("Send: %v", err)
			}
			for i, b := range p.got.Attachments[0].Blocks {
				if b.Text != nil {
					assertTextObject(t, "section text", i, b.Text.Text, textObjectLimit)
				}
				for _, f := range b.Fields {
					assertTextObject(t, "section field", i, f.Text, fieldTextLimit)
				}
				for _, e := range b.Elements {
					assertTextObject(t, "context element", i, e.Text, textObjectLimit)
				}
			}
			body, err := json.Marshal(p.got)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			wantJSON, err := json.Marshal(tc.preserved)
			if err != nil {
				t.Fatalf("marshal preserved: %v", err)
			}
			if !strings.Contains(string(body), strings.Trim(string(wantJSON), `"`)) {
				t.Fatalf("rendered message dropped the leading content of the oversized value")
			}
		})
	}
}

func assertTextObject(t *testing.T, kind string, block int, text string, limit int) {
	t.Helper()
	if !utf8.ValidString(text) {
		t.Fatalf("%s in block %d is not valid UTF-8", kind, block)
	}
	n := utf8.RuneCountInString(text)
	if n < 1 || n > limit {
		t.Fatalf("%s in block %d has %d characters, Slack accepts 1..%d", kind, block, n, limit)
	}
}
