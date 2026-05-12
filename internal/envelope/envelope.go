package envelope

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// defaultRegion is the fallback when no region can be derived from the
// message or any ARN field.
const defaultRegion = "us-east-1"

// snsMessagePrefix / snsMessageSuffix are the heuristic that decides
// whether to JSON-parse the SNS Message string.
const (
	snsMessagePrefix = "{"
	snsMessageSuffix = "}"
)

// Event is the normalized view over a single Lambda invocation record.
//
// It wraps the raw JSON payload (SNS record, EventBridge event, or direct
// invocation) and exposes typed accessors used by parser matchers and the
// router.
type Event struct {
	raw     json.RawMessage
	record  json.RawMessage
	message json.RawMessage
	subject string
	// snsTimestamp is the Sns.Timestamp field (RFC3339), used by Time() when
	// the inner message has no timestamp of its own.
	snsTimestamp string
	// eventSubscriptionArn is the SNS subscription ARN, used as the last
	// link in the ARN-derivation chain (message.resources[0] →
	// message.arn → record.EventSubscriptionArn).
	eventSubscriptionArn string
	// originalEnvelope is the unmodified outer envelope, used when fanning
	// SNS multi-record events into single-record copies.
	originalEnvelope json.RawMessage
}

// snsOuter mirrors the relevant fragments of a Lambda → SNS event envelope.
type snsOuter struct {
	Records []snsRecord `json:"Records"`
}

// snsRecord is one element of the SNS records slice. We model only the
// fields we read.
type snsRecord struct {
	EventSource          string          `json:"EventSource"`
	EventSubscriptionArn string          `json:"EventSubscriptionArn"`
	Sns                  json.RawMessage `json:"Sns"`
}

// snsBody is the Sns.* fragment we actually consume.
type snsBody struct {
	Message   string `json:"Message"`
	Subject   string `json:"Subject"`
	Timestamp string `json:"Timestamp"`
}

// New parses an outer Lambda payload (SNS, EventBridge, or direct
// invocation). Returns an error only when raw is empty or not valid JSON;
// shape-specific failures are tolerated and surfaced through the accessors.
func New(raw json.RawMessage) (*Event, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("empty payload")
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("payload is not valid json")
	}
	ev := &Event{raw: raw, originalEnvelope: raw}
	ev.populateFromOuter(raw)
	return ev, nil
}

// populateFromOuter applies the outer envelope to the Event:
//   - record = event.Records[0] when present, otherwise the event itself.
//   - message = record.Sns.Message (JSON-parsed when it looks like a JSON
//     object) when SNS, otherwise the record.
func (e *Event) populateFromOuter(raw json.RawMessage) {
	var outer snsOuter
	if err := json.Unmarshal(raw, &outer); err == nil && len(outer.Records) > 0 {
		e.applyRecord(outer.Records[0])
		return
	}
	// Not SNS — record == event, message == event.
	e.record = raw
	e.message = raw
}

// applyRecord decodes a single SNS record into the Event state.
func (e *Event) applyRecord(rec snsRecord) {
	recRaw, err := json.Marshal(rec)
	if err == nil {
		e.record = recRaw
	}
	e.eventSubscriptionArn = rec.EventSubscriptionArn

	if len(rec.Sns) == 0 {
		e.message = e.record
		return
	}
	var body snsBody
	if err := json.Unmarshal(rec.Sns, &body); err != nil {
		e.message = e.record
		return
	}
	e.subject = body.Subject
	e.snsTimestamp = body.Timestamp

	msg := strings.TrimSpace(body.Message)
	switch {
	case msg == "":
		e.message = e.record
	case strings.HasPrefix(msg, snsMessagePrefix) && strings.HasSuffix(msg, snsMessageSuffix) && json.Valid([]byte(msg)):
		e.message = json.RawMessage(msg)
	default:
		// Plain-text Message — wrap as a JSON string so the message stays a
		// valid json.RawMessage downstream consumers can detect via the
		// leading byte.
		quoted, qerr := json.Marshal(body.Message)
		if qerr != nil {
			e.message = e.record
			return
		}
		e.message = quoted
	}
}

// Records returns one Event per SNS record for multi-record SNS events; for
// EventBridge / direct payloads it returns a single-element slice
// containing the receiver.
func (e *Event) Records() []*Event {
	var outer snsOuter
	if err := json.Unmarshal(e.originalEnvelope, &outer); err != nil || len(outer.Records) <= 1 {
		return []*Event{e}
	}
	out := make([]*Event, 0, len(outer.Records))
	for _, rec := range outer.Records {
		single := &Event{
			raw:              e.originalEnvelope,
			originalEnvelope: e.originalEnvelope,
		}
		single.applyRecord(rec)
		out = append(out, single)
	}
	return out
}

// Raw returns the raw outer event JSON.
func (e *Event) Raw() json.RawMessage { return e.raw }

// Message returns the unwrapped inner message JSON. For SNS records this is
// the parsed Sns.Message (JSON when it parsed, a JSON-quoted string when it
// was plain text). For EventBridge / direct payloads it equals the outer
// event.
func (e *Event) Message() json.RawMessage { return e.message }

// Subject returns the SNS Subject when present, empty string otherwise.
func (e *Event) Subject() string { return e.subject }

// DetailType returns the EventBridge "detail-type" field, or empty when the
// envelope is not an EventBridge event.
func (e *Event) DetailType() string {
	return e.stringField("detail-type")
}

// Source returns the EventBridge source with the leading "aws." prefix
// stripped. Falls back to the ARN service segment when no source is set.
func (e *Event) Source() string {
	if src := e.stringField("source"); src != "" {
		return strings.TrimPrefix(src, "aws.")
	}
	return e.deriveARN().Service
}

// Region returns the AWS region for the event, following the fallback
// chain: message.region → arn region → "us-east-1".
func (e *Event) Region() string {
	if r := e.stringField("region"); r != "" {
		return r
	}
	if r := e.deriveARN().Region; r != "" {
		return r
	}
	return defaultRegion
}

// AccountID returns the AWS account that produced the event. Falls back to
// the ARN account segment when no explicit field is present.
func (e *Event) AccountID() string {
	if acc := e.stringField("account"); acc != "" {
		return acc
	}
	return e.deriveARN().AccountID
}

// Time returns the event timestamp. Precedence:
// message.time → message.Time → message.Timestamp → Sns.Timestamp →
// time.Now() as the last-resort default.
func (e *Event) Time() time.Time {
	for _, key := range []string{"time", "Time", "Timestamp"} {
		if v := e.stringField(key); v != "" {
			if t, ok := parseTime(v); ok {
				return t
			}
		}
	}
	if e.snsTimestamp != "" {
		if t, ok := parseTime(e.snsTimestamp); ok {
			return t
		}
	}
	return time.Now().UTC()
}

// Get returns the value at a top-level key on the inner message, or an
// empty RawMessage when missing.
func (e *Event) Get(path string) json.RawMessage {
	if len(e.message) == 0 || path == "" {
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(e.message, &obj); err != nil {
		return nil
	}
	return obj[path]
}

// stringField returns a top-level string field on the inner message.
func (e *Event) stringField(key string) string {
	raw := e.Get(key)
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// firstResource returns message.resources[0] when present.
func (e *Event) firstResource() string {
	raw := e.Get("resources")
	if len(raw) == 0 {
		return ""
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil || len(list) == 0 {
		return ""
	}
	return list[0]
}

// deriveARN walks the ARN-derivation chain: message.resources[0] →
// message.arn → record.EventSubscriptionArn.
func (e *Event) deriveARN() ARN {
	if arn := e.firstResource(); arn != "" {
		return ParseARN(arn)
	}
	if arn := e.stringField("arn"); arn != "" {
		return ParseARN(arn)
	}
	if e.eventSubscriptionArn != "" {
		return ParseARN(e.eventSubscriptionArn)
	}
	return ARN{}
}

// EventSubscriptionArn returns the SNS subscription ARN attached to the
// record (empty for non-SNS events). Parsers that emit "Received via" SNS
// footers consume this.
func (e *Event) EventSubscriptionArn() string { return e.eventSubscriptionArn }

// parseTime accepts the formats AWS emits across services (RFC3339 with
// nanos, RFC3339 with offsets like +0000, and the legacy CloudWatch alarm
// "2024-06-01T12:00:00.000+0000" shape).
func parseTime(s string) (time.Time, bool) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z0700",
		"2006-01-02T15:04:05Z0700",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
