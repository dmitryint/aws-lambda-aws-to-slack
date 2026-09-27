package slack

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

func TestTruncateText(t *testing.T) {
	const limit = 10
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"within_limit", "short", "short"},
		{"exactly_limit", "0123456789", "0123456789"},
		{"over_limit_ascii", "0123456789X", "012345678…"},
		{"over_limit_multibyte", strings.Repeat("é", 11), strings.Repeat("é", 9) + "…"},
		{"cut_inside_link_backs_off_to_open", "ab <https://x|label> tail", "ab …"},
		{"link_at_start_spanning_cut", "<https://example.com|x>", "…"},
		{"closed_link_before_cut_kept", "<u|l> abcdefgh", "<u|l> abc…"},
		{"unterminated_angle_cut_normally", "a <bcdefghijk", "a <bcdefg…"},
		{"stray_angle_before_later_link_cut_normally", "x <yyyyyyyy <u|l>", "x <yyyyyy…"},
		{"stray_angle_then_link_spanning_cut", "a < b <u|l>xyz", "a < b …"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateText(tc.in, limit)
			if got != tc.want {
				t.Fatalf("truncateText(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("truncateText(%q) produced invalid UTF-8", tc.in)
			}
			if n := utf8.RuneCountInString(got); n > limit {
				t.Fatalf("truncateText(%q) has %d runes, limit %d", tc.in, n, limit)
			}
		})
	}
}

func TestImageBlock_TruncatesAltText(t *testing.T) {
	got := ImageBlock("https://example/x.png", strings.Repeat("a", maxImageAltTextLength+500))
	if n := utf8.RuneCountInString(got.AltText); n != maxImageAltTextLength {
		t.Fatalf("AltText has %d runes, want %d", n, maxImageAltTextLength)
	}
	if !strings.HasSuffix(got.AltText, truncationMarker) {
		t.Fatalf("AltText missing truncation marker: %q", got.AltText[len(got.AltText)-10:])
	}
}

func TestFieldsSection_TruncatesOversizedFieldWithoutMutatingInput(t *testing.T) {
	long := "*Resource*\n" + strings.Repeat("x", maxFieldTextLength+50)
	fields := []TextObject{
		{Type: TextTypeMrkdwn, Text: "*k1*\nv1"},
		{Type: TextTypeMrkdwn, Text: long},
	}
	got := FieldsSection(fields)
	if got.Fields[0].Text != "*k1*\nv1" {
		t.Fatalf("within-limit field changed: %q", got.Fields[0].Text)
	}
	if n := utf8.RuneCountInString(got.Fields[1].Text); n != maxFieldTextLength {
		t.Fatalf("oversized field has %d runes, want %d", n, maxFieldTextLength)
	}
	if fields[1].Text != long {
		t.Fatalf("FieldsSection mutated the caller's slice")
	}
}

func TestRenderer_ImageAltTextWithinLimit(t *testing.T) {
	p := &recordingPoster{}
	title := strings.Repeat("t", maxImageAltTextLength+100)
	if err := NewRenderer(p).Send(t.Context(), &notify.Notification{
		Severity: notify.SeverityWarning,
		Title:    title,
		ImageURL: "https://img/chart.png",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	var image *Block
	for i, b := range p.got.Attachments[0].Blocks {
		if b.Type == BlockTypeImage {
			image = &p.got.Attachments[0].Blocks[i]
		}
	}
	if image == nil {
		t.Fatalf("image block missing")
	}
	if n := utf8.RuneCountInString(image.AltText); n < 1 || n > maxImageAltTextLength {
		t.Fatalf("alt_text has %d characters, Slack accepts 1..%d", n, maxImageAltTextLength)
	}
	if !strings.HasPrefix(image.AltText, title[:200]) {
		t.Fatalf("alt_text dropped the leading title content")
	}
}

func TestRenderer_SummaryNotCutInsideLink(t *testing.T) {
	p := &recordingPoster{}
	lead := strings.Repeat("a", maxSectionTextLength-10)
	if err := NewRenderer(p).Send(t.Context(), &notify.Notification{
		Severity: notify.SeverityNotice,
		Title:    "T",
		Summary:  lead + notify.Link("https://console.aws.amazon.com/guardduty/home", "console"),
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	got := p.got.Attachments[0].Blocks[1].Text.Text
	if want := lead + truncationMarker; got != want {
		t.Fatalf("summary tail = %q, want %q", got[len(got)-20:], want[len(want)-20:])
	}
}
