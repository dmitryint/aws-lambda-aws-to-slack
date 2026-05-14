package slack

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

// recordingPoster captures the last Message handed to Post.
type recordingPoster struct {
	got  *Message
	err  error
	hits int
}

func (r *recordingPoster) Post(_ context.Context, m *Message) error {
	r.hits++
	r.got = m
	return r.err
}

func TestRenderer_SeverityVisualMapping(t *testing.T) {
	cases := []struct {
		sev       notify.Severity
		wantColor string
		wantEmoji string
	}{
		{notify.SeverityCritical, "danger", ":red_circle:"},
		{notify.SeverityWarning, "warning", ":large_yellow_circle:"},
		{notify.SeverityOK, "good", ":large_green_circle:"},
		{notify.SeverityNotice, "#439FE0", ":large_blue_circle:"},
		{notify.SeverityInfo, "#dddddd", ":white_circle:"},
		{notify.SeverityUnknown, "#dddddd", ":white_circle:"},
	}
	for _, tc := range cases {
		t.Run(tc.sev.String(), func(t *testing.T) {
			p := &recordingPoster{}
			r := NewRenderer(p)
			n := &notify.Notification{
				Source:   "test",
				Severity: tc.sev,
				Title:    "headline",
				Subtitle: "sub",
				Fallback: "fb",
			}
			if err := r.Send(t.Context(), n); err != nil {
				t.Fatalf("Send: %v", err)
			}
			if p.hits != 1 {
				t.Fatalf("Post hits = %d, want 1", p.hits)
			}
			att := p.got.Attachments[0]
			if att.Color != tc.wantColor {
				t.Fatalf("color = %q, want %q", att.Color, tc.wantColor)
			}
			header := att.Blocks[0]
			if header.Text == nil || !strings.HasPrefix(header.Text.Text, tc.wantEmoji+" ") {
				t.Fatalf("header missing emoji %q at start: %+v", tc.wantEmoji, header.Text)
			}
		})
	}
}

func TestRenderer_HeaderLinkedAndUnlinked(t *testing.T) {
	t.Run("with_url", func(t *testing.T) {
		p := &recordingPoster{}
		_ = NewRenderer(p).Send(t.Context(), &notify.Notification{
			Severity: notify.SeverityCritical,
			Title:    "alarm",
			TitleURL: "https://example/alarm",
		})
		got := p.got.Attachments[0].Blocks[0].Text.Text
		want := ":red_circle: *<https://example/alarm|alarm>*"
		if got != want {
			t.Fatalf("header = %q, want %q", got, want)
		}
	})
	t.Run("without_url", func(t *testing.T) {
		p := &recordingPoster{}
		_ = NewRenderer(p).Send(t.Context(), &notify.Notification{
			Severity: notify.SeverityOK,
			Title:    "ok-event",
		})
		got := p.got.Attachments[0].Blocks[0].Text.Text
		want := ":large_green_circle: *ok-event*"
		if got != want {
			t.Fatalf("header = %q, want %q", got, want)
		}
	})
}

func TestRenderer_BlockLayout(t *testing.T) {
	p := &recordingPoster{}
	_ = NewRenderer(p).Send(t.Context(), &notify.Notification{
		Severity: notify.SeverityWarning,
		Title:    "T",
		Subtitle: "ST",
		Summary:  "summary body",
		Fields: []notify.Field{
			{Key: "k1", Value: "v1"},
			{Key: "k2", Value: "v2"},
		},
		ImageURL:  "https://img/chart.png",
		Footnotes: []string{"footer-a", "footer-b"},
	})
	blocks := p.got.Attachments[0].Blocks
	wantTypes := []string{
		BlockTypeSection, // header
		BlockTypeSection, // summary
		BlockTypeSection, // fields
		BlockTypeImage,   // chart
		BlockTypeContext, // footnotes
	}
	gotTypes := make([]string, 0, len(blocks))
	for _, b := range blocks {
		gotTypes = append(gotTypes, b.Type)
	}
	if diff := cmp.Diff(wantTypes, gotTypes); diff != "" {
		t.Fatalf("block types (-want +got):\n%s", diff)
	}
}

func TestRenderer_OmitsEmptyOptionalBlocks(t *testing.T) {
	p := &recordingPoster{}
	_ = NewRenderer(p).Send(t.Context(), &notify.Notification{
		Severity: notify.SeverityOK,
		Title:    "T",
	})
	blocks := p.got.Attachments[0].Blocks
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block (header only), got %d: %+v", len(blocks), blocks)
	}
}

func TestRenderer_FieldsChunkedAtSlackLimit(t *testing.T) {
	p := &recordingPoster{}
	fields := make([]notify.Field, 14)
	for i := range fields {
		fields[i] = notify.Field{Key: "k", Value: "v"}
	}
	_ = NewRenderer(p).Send(t.Context(), &notify.Notification{
		Severity: notify.SeverityCritical,
		Title:    "T",
		Fields:   fields,
	})
	blocks := p.got.Attachments[0].Blocks
	fieldSections := 0
	for _, b := range blocks {
		if b.Type == BlockTypeSection && len(b.Fields) > 0 {
			fieldSections++
			if len(b.Fields) > MaxFieldsPerSection {
				t.Fatalf("field section exceeds Slack cap: %d", len(b.Fields))
			}
		}
	}
	if fieldSections != 2 {
		t.Fatalf("expected 2 chunked field sections, got %d", fieldSections)
	}
}

func TestRenderer_Accepts_MinSeverityFloor(t *testing.T) {
	r := NewRenderer(&recordingPoster{}).WithMinSeverity(notify.SeverityWarning)
	cases := map[notify.Severity]bool{
		notify.SeverityInfo:     false,
		notify.SeverityOK:       false,
		notify.SeverityNotice:   false,
		notify.SeverityWarning:  true,
		notify.SeverityCritical: true,
	}
	for s, want := range cases {
		if got := r.Accepts(s); got != want {
			t.Fatalf("Accepts(%s) = %v, want %v", s, got, want)
		}
	}
}

func TestRenderer_Name(t *testing.T) {
	if got := (Renderer{}).Name(); got != "slack" {
		t.Fatalf("Name = %q", got)
	}
}

func TestRenderer_PropagatesClientError(t *testing.T) {
	wantErr := errors.New("boom")
	p := &recordingPoster{err: wantErr}
	err := NewRenderer(p).Send(t.Context(), &notify.Notification{Severity: notify.SeverityOK, Title: "x"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestRenderer_Send_NilNotificationIsNoop(t *testing.T) {
	p := &recordingPoster{}
	if err := NewRenderer(p).Send(t.Context(), nil); err != nil {
		t.Fatalf("Send(nil) error: %v", err)
	}
	if p.hits != 0 {
		t.Fatalf("nil notification must not call Post")
	}
}

func TestStripMrkdwnLinks(t *testing.T) {
	cases := map[string]string{
		"plain":                             "plain",
		"see <https://x|the docs> please":   "see the docs please",
		"first <https://a|A>, second <b|B>": "first A, second B",
		"<bare-url-no-pipe>":                "bare-url-no-pipe",
		"unterminated <oh no":               "unterminated <oh no",
		"":                                  "",
	}
	for in, want := range cases {
		if got := stripMrkdwnLinks(in); got != want {
			t.Fatalf("stripMrkdwnLinks(%q) = %q, want %q", in, got, want)
		}
	}
}
