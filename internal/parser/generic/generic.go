package generic

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

// name is the stable identifier the router uses for log lines.
const name = "generic"

// unknownAuthor is the placeholder shown when no EventBridge source /
// ARN service is derivable.
const unknownAuthor = "<unknown>"

// defaultTitle is the fallback used when there is no SNS Subject and no
// EventBridge "detail-type".
const defaultTitle = "Raw Event"

// shortFieldCutoff is the value-length below which a field is rendered as
// "short" (two-column layout) — same heuristic Slack itself uses inside
// legacy fields.
const shortFieldCutoff = 40

// maxFields caps how many top-level keys are rendered as discrete fields
// before falling back to a JSON code-block.
const maxFields = 8

// Parser is the always-matching catch-all. It runs last in the waterfall
// and ensures every Lambda invocation produces some Slack output, even for
// payloads no specialized parser claims.
type Parser struct{}

// New returns a Parser ready to register with the router.
func New() *Parser { return &Parser{} }

// Name returns the stable parser identifier.
func (Parser) Name() string { return name }

// Match always returns true — this parser is the catch-all.
func (Parser) Match(_ *envelope.Event) bool { return true }

// Parse renders a Notification describing the raw event. Four cases:
//   - EventBridge envelope (object with `source` / `detail-type`).
//   - SNS string body (free-form text published via SNS).
//   - SNS JSON body (parsed object delivered through SNS).
//   - Non-object message (anything else — render as a string).
func (Parser) Parse(_ context.Context, e *envelope.Event) (*notify.Notification, error) {
	title := e.Subject()
	if title == "" {
		title = defaultTitle
	}
	author := e.Source()
	if author == "" {
		author = unknownAuthor
	}

	body, asObject, err := decodeMessage(e.Message())
	if err != nil {
		return nil, fmt.Errorf("decode message: %w", err)
	}

	n := &notify.Notification{
		Source:   name,
		Severity: notify.SeverityInfo,
		Fallback: fallback(e),
	}

	if !asObject {
		n.Title = title
		n.Subtitle = author
		if body.text != "" {
			n.Summary = codeBlock(body.text)
		}
		return n, nil
	}

	// Object path — strip the keys that are hoisted into the header.
	headerAuthor := extractAuthor(body.object, author)
	if dt := extractStringKey(body.object, "detail-type"); dt != "" {
		title = dt
	}
	delete(body.object, "time")

	n.Title = title
	n.Subtitle = headerAuthor
	if len(body.object) > 0 && len(body.object) <= maxFields {
		if fields := buildFields(body.object); len(fields) > 0 {
			n.Fields = fields
			return n, nil
		}
	}
	pretty, err := json.MarshalIndent(body.object, "", "  ")
	if err == nil {
		n.Summary = codeBlock(string(pretty))
	}
	return n, nil
}

// decoded carries the outcome of decodeMessage: either a parsed object or
// a literal string (text). asObject distinguishes the two cases.
type decoded struct {
	object map[string]any
	text   string
}

// decodeMessage parses the inner message. It accepts JSON objects, JSON
// strings, and JSON primitives, returning whether the body is an object
// (asObject true) or a literal string.
func decodeMessage(raw json.RawMessage) (decoded, bool, error) {
	if len(raw) == 0 {
		return decoded{text: ""}, false, nil
	}
	// Try object first.
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil && obj != nil {
		return decoded{object: obj}, true, nil
	}
	// Fall back to string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return decoded{text: s}, false, nil
	}
	// Last-resort: treat the raw bytes as a string.
	return decoded{text: string(raw)}, false, nil
}

// extractAuthor builds the author_name from `msg.source / msg.region /
// msg.account`. The matched keys are deleted from obj so they don't repeat
// in the rendered body.
func extractAuthor(obj map[string]any, fallback string) string {
	src := extractStringKey(obj, "source")
	if src == "" {
		return fallback
	}
	parts := make([]string, 0, 2)
	if region := extractStringKey(obj, "region"); region != "" {
		parts = append(parts, region)
	}
	if acc := extractStringKey(obj, "account"); acc != "" {
		parts = append(parts, acc)
	}
	if len(parts) == 0 {
		return src
	}
	return fmt.Sprintf("%s (%s)", src, strings.Join(parts, " - "))
}

// extractStringKey returns the string value at key and deletes the key
// from the map. Non-string values are left in place (the caller will
// render them through the generic fallback).
func extractStringKey(obj map[string]any, key string) string {
	v, ok := obj[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	delete(obj, key)
	return s
}

// buildFields converts the message's top-level keys into Notification fields.
// The key order is stabilized so golden files are deterministic.
func buildFields(obj map[string]any) []notify.Field {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fields := make([]notify.Field, 0, len(keys))
	for _, k := range keys {
		v := obj[k]
		// "version" key with an empty value: skip.
		if k == "version" && isEmpty(v) {
			continue
		}
		fields = append(fields, notify.Field{Key: k, Value: stringifyValue(v)})
	}
	return fields
}

// stringifyValue renders a value for a field row:
//   - strings pass through.
//   - one-element string arrays unwrap to the element.
//   - everything else is JSON-marshaled.
func stringifyValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		if len(t) == 1 {
			if s, ok := t[0].(string); ok {
				return s
			}
		}
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	}
}

// isEmpty returns true for the values treated as empty inside the
// `if (key == "version" && !val)` short-circuit.
func isEmpty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case bool:
		return !t
	case float64:
		return t == 0
	default:
		return false
	}
}

// codeBlock wraps text in a triple-backtick mrkdwn fence.
func codeBlock(text string) string {
	return "```\n" + text + "\n```"
}

// fallback renders the plain-text fallback for email digests, mobile push,
// and ancient Slack clients. The entire outer record is pretty-printed.
func fallback(e *envelope.Event) string {
	pretty, err := json.MarshalIndent(e.Raw(), "", "  ")
	if err != nil {
		return string(e.Raw())
	}
	return string(pretty)
}

// shortFieldCutoff is currently informational; the constant stays in the
// package for tests that may probe the threshold.
var _ = shortFieldCutoff
