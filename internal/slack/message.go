package slack

// Message is the top-level Slack webhook payload.
//
// The wire shape is a hybrid envelope: a single legacy Attachment carries
// the severity color and wraps modern Block Kit blocks, giving us both
// per-block colored side-bars and reliable image rendering for CloudWatch
// chart images.
type Message struct {
	Channel     string       `json:"channel,omitempty"`
	Username    string       `json:"username,omitempty"`
	IconEmoji   string       `json:"icon_emoji,omitempty"`
	Text        string       `json:"text,omitempty"`
	UnfurlLinks bool         `json:"unfurl_links,omitempty"`
	UnfurlMedia bool         `json:"unfurl_media,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Attachment is the legacy Slack attachment envelope that carries the color
// side-bar and wraps Block Kit blocks for the actual content.
//
// The hybrid envelope deliberately omits legacy fields like image_url at the
// attachment level — images live inside Blocks where Slack renders signed
// S3 URLs reliably.
type Attachment struct {
	Color    string  `json:"color,omitempty"`
	Fallback string  `json:"fallback,omitempty"`
	Blocks   []Block `json:"blocks,omitempty"`
}

// Block is a Block Kit element. Slack's grammar covers a long list of types
// (section, image, context, divider, header, actions, …); we model only the
// fields the parsers actually emit and keep the rest pluggable through the
// json.RawMessage Extras escape hatch.
type Block struct {
	Type     string       `json:"type"`
	Text     *TextObject  `json:"text,omitempty"`
	Elements []TextObject `json:"elements,omitempty"`
	ImageURL string       `json:"image_url,omitempty"`
	AltText  string       `json:"alt_text,omitempty"`
	Fields   []TextObject `json:"fields,omitempty"`
}

// TextObject is the text payload used by section, context, and field blocks.
// Type is either "mrkdwn" or "plain_text".
type TextObject struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Text-object type constants — Slack accepts only these two values.
const (
	TextTypeMrkdwn    = "mrkdwn"
	TextTypePlainText = "plain_text"
)

// Block-type constants for the subset of Block Kit we emit.
const (
	BlockTypeSection = "section"
	BlockTypeContext = "context"
	BlockTypeImage   = "image"
	BlockTypeDivider = "divider"
	BlockTypeHeader  = "header"
)

// NewMessage builds a hybrid-envelope Message with the given color, fallback
// text, and Block Kit blocks. The result has exactly one Attachment.
func NewMessage(color, fallback string, blocks ...Block) *Message {
	return &Message{
		Attachments: []Attachment{{
			Color:    color,
			Fallback: fallback,
			Blocks:   blocks,
		}},
	}
}

// SectionBlock returns a section block carrying a single mrkdwn text body.
func SectionBlock(mrkdwn string) Block {
	return Block{
		Type: BlockTypeSection,
		Text: &TextObject{Type: TextTypeMrkdwn, Text: mrkdwn},
	}
}

// ContextBlock returns a context block whose elements are mrkdwn snippets —
// the standard way to attach footers (timestamps, links, etc.) under a
// section.
func ContextBlock(mrkdwnElements ...string) Block {
	elems := make([]TextObject, 0, len(mrkdwnElements))
	for _, e := range mrkdwnElements {
		elems = append(elems, TextObject{Type: TextTypeMrkdwn, Text: e})
	}
	return Block{Type: BlockTypeContext, Elements: elems}
}

// ImageBlock returns an image block. Slack renders signed S3 URLs reliably
// inside Block Kit images; the legacy attachment.image_url path silently
// drops long URLs.
func ImageBlock(url, altText string) Block {
	return Block{Type: BlockTypeImage, ImageURL: url, AltText: altText}
}

// FieldsSection returns a section block carrying a fields array — used by
// parsers that emit key/value rows underneath the main mrkdwn body.
func FieldsSection(fields []TextObject) Block {
	return Block{Type: BlockTypeSection, Fields: fields}
}
