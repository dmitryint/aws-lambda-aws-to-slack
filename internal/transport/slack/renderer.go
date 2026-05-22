package slack

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

// Renderer is the Slack transport: it translates a notify.Notification into
// a Block Kit Message and posts it to the configured webhook.
//
// Renderer is the *single* place that maps Severity to visual cues (color
// side-bar, header emoji, attachment styling). Parsers must not duplicate
// any of this mapping locally — Slack rendering drift is fixed once here.
type Renderer struct {
	client      Poster
	minSeverity notify.Severity
}

// Poster is the minimal interface a Renderer needs from a Slack client.
// Production wires *Client; tests substitute a recorder.
type Poster interface {
	Post(ctx context.Context, m *Message) error
}

// NewRenderer returns a Renderer that posts via the given client. All
// severities are accepted unless WithMinSeverity is applied.
func NewRenderer(client Poster) *Renderer {
	return &Renderer{client: client, minSeverity: notify.SeverityUnknown}
}

// WithMinSeverity drops Notifications whose Severity is strictly less than
// the configured floor. Default behavior (when unset) is to accept every
// severity, including SeverityUnknown.
func (r *Renderer) WithMinSeverity(minSev notify.Severity) *Renderer {
	r.minSeverity = minSev
	return r
}

// Name is the renderer identifier used in handler logs and metrics.
func (Renderer) Name() string { return "slack" }

// Accepts returns true when the Renderer should receive a Notification of
// the given Severity.
func (r *Renderer) Accepts(s notify.Severity) bool {
	return s >= r.minSeverity
}

// Send renders the Notification as a Slack Message and POSTs it.
func (r *Renderer) Send(ctx context.Context, n *notify.Notification) error {
	if n == nil {
		return nil
	}
	msg := r.build(n)
	return r.client.Post(ctx, msg)
}

// LogPost mirrors *Client.LogPost so the Renderer can be used in fan-out
// pipelines that record failures without aborting the batch.
func (r *Renderer) LogPost(ctx context.Context, n *notify.Notification, logger *slog.Logger) error {
	if err := r.Send(ctx, n); err != nil {
		logger.ErrorContext(ctx, "slack render failed",
			"err", err,
			"source", n.Source,
			"severity", n.Severity.String(),
		)
		return err
	}
	return nil
}

// build is the only place Severity maps to Slack visuals.
//
//	Critical → danger   + 🔴 (:red_circle:)
//	Warning  → warning  + 🟡 (:large_yellow_circle:)
//	OK       → good     + 🟢 (:large_green_circle:)
//	Notice   → accent   + 🔵 (:large_blue_circle:)
//	Info     → neutral  + ⚪ (:white_circle:)
//	Unknown  → neutral  + ⚪
//
// The attachment color is still emitted for any client that does render the
// legacy side-bar; the emoji prefix is the always-on cue.
func (r *Renderer) build(n *notify.Notification) *Message {
	color := severityColor(n.Severity)
	emoji := severityEmoji(n.Severity)

	view := *n
	if hideLinksEnabled() {
		view = stripLinks(view)
	}

	blocks := []Block{SectionBlock(buildHeaderText(emoji, &view))}
	if view.Summary != "" {
		blocks = append(blocks, SectionBlock(view.Summary))
	}
	if len(view.Fields) > 0 {
		blocks = append(blocks, FieldsSections(toTextObjects(view.Fields))...)
	}
	if view.ImageURL != "" {
		blocks = append(blocks, ImageBlock(view.ImageURL, view.Title+" chart"))
	}
	if len(view.Footnotes) > 0 {
		blocks = append(blocks, ContextBlock(view.Footnotes...))
	}

	return NewMessage(color, view.Fallback, blocks...)
}

// stripLinks returns a copy of the notification with every mrkdwn link
// segment rewritten to its plain label. The TitleURL is also blanked so
// the header text falls through to the bare Title.
func stripLinks(n notify.Notification) notify.Notification {
	n.TitleURL = ""
	n.Title = stripMrkdwnLinks(n.Title)
	n.Subtitle = stripMrkdwnLinks(n.Subtitle)
	n.Summary = stripMrkdwnLinks(n.Summary)
	if len(n.Fields) > 0 {
		fields := make([]notify.Field, len(n.Fields))
		for i, f := range n.Fields {
			fields[i] = notify.Field{Key: f.Key, Value: stripMrkdwnLinks(f.Value)}
		}
		n.Fields = fields
	}
	if len(n.Footnotes) > 0 {
		notes := make([]string, len(n.Footnotes))
		for i, line := range n.Footnotes {
			notes[i] = stripMrkdwnLinks(line)
		}
		n.Footnotes = notes
	}
	return n
}

// buildHeaderText assembles `<emoji> *<TitleLink|Title>*\n_<Subtitle>_`.
// The subtitle line is omitted when Subtitle is empty so simple
// notifications stay compact.
func buildHeaderText(emoji string, n *notify.Notification) string {
	var titleSegment string
	if n.TitleURL != "" {
		titleSegment = notify.Link(n.TitleURL, n.Title)
	} else {
		titleSegment = n.Title
	}
	header := fmt.Sprintf("%s *%s*", emoji, titleSegment)
	if n.Subtitle != "" {
		header += "\n_" + n.Subtitle + "_"
	}
	return header
}

// toTextObjects projects notify.Field rows into mrkdwn TextObjects shaped
// for a section's fields array.
func toTextObjects(fields []notify.Field) []TextObject {
	out := make([]TextObject, 0, len(fields))
	for _, f := range fields {
		out = append(out, TextObject{
			Type: TextTypeMrkdwn,
			Text: "*" + f.Key + "*\n" + f.Value,
		})
	}
	return out
}

// severityColor maps a notify.Severity to a Slack attachment color.
func severityColor(s notify.Severity) string {
	switch s {
	case notify.SeverityCritical:
		return ColorCritical
	case notify.SeverityWarning:
		return ColorWarning
	case notify.SeverityOK:
		return ColorOK
	case notify.SeverityNotice:
		return ColorAccent
	default:
		return ColorNeutral
	}
}

// severityEmoji maps a notify.Severity to the in-content emoji shortcode
// that guarantees visual differentiation regardless of how the workspace
// renders the legacy attachment color side-bar.
func severityEmoji(s notify.Severity) string {
	switch s {
	case notify.SeverityCritical:
		return ":red_circle:"
	case notify.SeverityWarning:
		return ":large_yellow_circle:"
	case notify.SeverityOK:
		return ":large_green_circle:"
	case notify.SeverityNotice:
		return ":large_blue_circle:"
	default:
		return ":white_circle:"
	}
}

// stripMrkdwnLinks rewrites `<url|label>` segments to the plain label, so
// callers that need a plain-text fallback can derive one from the same
// Summary string the Slack renderer uses.
func stripMrkdwnLinks(s string) string {
	var b strings.Builder
	for {
		open := strings.IndexByte(s, '<')
		if open == -1 {
			b.WriteString(s)
			return b.String()
		}
		closeIdx := strings.IndexByte(s[open:], '>')
		if closeIdx == -1 {
			b.WriteString(s)
			return b.String()
		}
		closeIdx += open
		b.WriteString(s[:open])
		segment := s[open+1 : closeIdx]
		if pipe := strings.IndexByte(segment, '|'); pipe != -1 {
			b.WriteString(segment[pipe+1:])
		} else {
			b.WriteString(segment)
		}
		s = s[closeIdx+1:]
	}
}
