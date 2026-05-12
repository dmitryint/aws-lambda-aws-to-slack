package slack

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewMessage_WrapsBlocksInSingleAttachment(t *testing.T) {
	m := NewMessage(ColorOK, "fallback", SectionBlock("body"))
	if len(m.Attachments) != 1 {
		t.Fatalf("want 1 attachment, got %d", len(m.Attachments))
	}
	att := m.Attachments[0]
	if att.Color != ColorOK {
		t.Fatalf("Color: %q", att.Color)
	}
	if att.Fallback != "fallback" {
		t.Fatalf("Fallback: %q", att.Fallback)
	}
	if len(att.Blocks) != 1 || att.Blocks[0].Type != BlockTypeSection {
		t.Fatalf("blocks: %+v", att.Blocks)
	}
}

func TestSectionBlock_MrkdwnText(t *testing.T) {
	got := SectionBlock("hello *world*")
	want := Block{
		Type: BlockTypeSection,
		Text: &TextObject{Type: TextTypeMrkdwn, Text: "hello *world*"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("SectionBlock (-want +got):\n%s", diff)
	}
}

func TestContextBlock_PreservesElementOrder(t *testing.T) {
	got := ContextBlock("one", "two", "three")
	if got.Type != BlockTypeContext {
		t.Fatalf("Type: %q", got.Type)
	}
	if len(got.Elements) != 3 {
		t.Fatalf("Elements len: %d", len(got.Elements))
	}
	for i, want := range []string{"one", "two", "three"} {
		if got.Elements[i].Text != want {
			t.Fatalf("Elements[%d]: %q", i, got.Elements[i].Text)
		}
		if got.Elements[i].Type != TextTypeMrkdwn {
			t.Fatalf("Elements[%d].Type: %q", i, got.Elements[i].Type)
		}
	}
}

func TestImageBlock_FieldsSet(t *testing.T) {
	got := ImageBlock("https://example/x.png", "alt")
	want := Block{
		Type:     BlockTypeImage,
		ImageURL: "https://example/x.png",
		AltText:  "alt",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("ImageBlock (-want +got):\n%s", diff)
	}
}

func TestFieldsSection_Wrapping(t *testing.T) {
	fields := []TextObject{
		{Type: TextTypeMrkdwn, Text: "*k1*\nv1"},
		{Type: TextTypeMrkdwn, Text: "*k2*\nv2"},
	}
	got := FieldsSection(fields)
	if got.Type != BlockTypeSection {
		t.Fatalf("Type: %q", got.Type)
	}
	if !cmp.Equal(fields, got.Fields) {
		t.Fatalf("Fields mismatch: %v", got.Fields)
	}
}

func TestMessageJSON_OmitsEmptyFields(t *testing.T) {
	m := NewMessage(ColorOK, "fb", SectionBlock("hi"))
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	for _, banned := range []string{"channel", "username", "icon_emoji"} {
		if strings.Contains(s, banned) {
			t.Fatalf("output should omit %q: %s", banned, s)
		}
	}
	if !strings.Contains(s, `"color":"good"`) {
		t.Fatalf("color missing: %s", s)
	}
}
