package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestSeverity_String(t *testing.T) {
	cases := map[Severity]string{
		SeverityUnknown:  "unknown",
		SeverityInfo:     "info",
		SeverityOK:       "ok",
		SeverityNotice:   "notice",
		SeverityWarning:  "warning",
		SeverityCritical: "critical",
		Severity(99):     "unknown",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Fatalf("Severity(%d).String() = %q, want %q", s, got, want)
		}
	}
}

func TestSeverity_OrderedAscending(t *testing.T) {
	got := []Severity{
		SeverityUnknown, SeverityInfo, SeverityOK, SeverityNotice,
		SeverityWarning, SeverityCritical,
	}
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("severity ordering broken at index %d: %v", i, got)
		}
	}
}

func TestLink(t *testing.T) {
	cases := []struct {
		name       string
		url, label string
		want       string
	}{
		{"both", "https://example", "label", "<https://example|label>"},
		{"empty_label_uses_url", "https://example", "", "<https://example|https://example>"},
		{"empty_url_returns_label", "", "label", "label"},
		{"both_empty", "", "", ""},
		{"strips_pipe_in_url", "https://x|y", "label", "<https://xy|label>"},
		{"strips_pipe_in_label", "https://x", "a|b", "<https://x|ab>"},
		{"strips_pipe_in_bare_label", "", "a|b", "ab"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Link(tc.url, tc.label); got != tc.want {
				t.Fatalf("Link(%q, %q) = %q, want %q", tc.url, tc.label, got, tc.want)
			}
		})
	}
}

func TestNotification_DedupRelease(t *testing.T) {
	plain := &Notification{Source: "s", Title: "t"}
	if got := plain.DedupKey(); got != "" {
		t.Fatalf("DedupKey without release = %q, want empty", got)
	}
	if err := plain.ReleaseDedup(t.Context()); err != nil {
		t.Fatalf("ReleaseDedup without release: %v", err)
	}

	var released string
	hooked := &Notification{Source: "s", Title: "t"}
	hooked.SetDedupRelease("k", func(_ context.Context, key string) error {
		released = key
		return nil
	})
	if got := hooked.DedupKey(); got != "k" {
		t.Fatalf("DedupKey = %q, want k", got)
	}
	if err := hooked.ReleaseDedup(t.Context()); err != nil {
		t.Fatalf("ReleaseDedup: %v", err)
	}
	if released != "k" {
		t.Fatalf("released key = %q, want k", released)
	}

	plainJSON, err := json.Marshal(plain)
	if err != nil {
		t.Fatalf("marshal plain: %v", err)
	}
	hookedJSON, err := json.Marshal(hooked)
	if err != nil {
		t.Fatalf("marshal hooked: %v", err)
	}
	if !bytes.Equal(hookedJSON, plainJSON) {
		t.Fatalf("dedup release leaked into JSON:\n got %s\nwant %s", hookedJSON, plainJSON)
	}
}
