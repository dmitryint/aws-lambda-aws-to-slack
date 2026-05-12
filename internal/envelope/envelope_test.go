package envelope

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

const samplesRoot = "../../samples"

func TestNew_RejectsEmptyOrInvalid(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{name: "empty", in: ""},
		{name: "whitespace", in: "   \n  "},
		{name: "not-json", in: "this is not json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(json.RawMessage(tc.in)); err == nil {
				t.Fatalf("expected error for %q, got nil", tc.in)
			}
		})
	}
}

func TestNew_SNSJSONStringMessage(t *testing.T) {
	raw := readSample(t, "sns_envelope/record_with_json_string.json")
	ev, err := New(raw)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if ev.Subject() != `ALARM: "example-alarm" in US East (Ohio)` {
		t.Fatalf("Subject mismatch: %q", ev.Subject())
	}
	var msg map[string]any
	if err := json.Unmarshal(ev.Message(), &msg); err != nil {
		t.Fatalf("inner message did not parse as JSON: %v", err)
	}
	if got := msg["AlarmName"]; got != "example-alarm" {
		t.Fatalf("AlarmName mismatch: %v", got)
	}
	if got := ev.EventSubscriptionArn(); got != "arn:aws:sns:us-east-2:123456789012:alarm-topic:21be56ed-a058-49f5-8c98-aedd2564c486" {
		t.Fatalf("EventSubscriptionArn: %s", got)
	}
}

func TestNew_SNSPlainTextMessage(t *testing.T) {
	raw := readSample(t, "sns_envelope/record_with_plain_text.json")
	ev, err := New(raw)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Plain-text message is JSON-encoded as a string for downstream consumers.
	var s string
	if err := json.Unmarshal(ev.Message(), &s); err != nil {
		t.Fatalf("inner message should be a JSON string: %v", err)
	}
	if want := "StackName='my-stack'"; !contains(s, want) {
		t.Fatalf("plain-text body missing %q: %s", want, s)
	}
	if ev.Subject() != "AWS CloudFormation Notification" {
		t.Fatalf("Subject mismatch: %q", ev.Subject())
	}
}

func TestNew_SNSSingleRecord(t *testing.T) {
	raw := readSample(t, "sns_envelope/single_record.json")
	ev, err := New(raw)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	recs := ev.Records()
	if len(recs) != 1 {
		t.Fatalf("Records: want 1, got %d", len(recs))
	}
	if recs[0].Subject() != "TestInvoke" {
		t.Fatalf("Subject: %q", recs[0].Subject())
	}
}

func TestRecords_MultiRecordSplit(t *testing.T) {
	raw := readSample(t, "sns_envelope/multi_record.json")
	ev, err := New(raw)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	recs := ev.Records()
	if len(recs) != 2 {
		t.Fatalf("Records: want 2, got %d", len(recs))
	}
	if recs[0].Subject() != "TestInvoke1" || recs[1].Subject() != "TestInvoke2" {
		t.Fatalf("Subjects: %q / %q", recs[0].Subject(), recs[1].Subject())
	}
	// Per-record SNS timestamp is the precedence fallback for Time().
	want0 := mustTime(t, "2019-01-02T12:45:07Z")
	want1 := mustTime(t, "2019-01-02T12:45:08Z")
	if diff := cmp.Diff(want0, recs[0].Time(), cmpopts.EquateApproxTime(time.Second)); diff != "" {
		t.Fatalf("record[0] Time mismatch:\n%s", diff)
	}
	if diff := cmp.Diff(want1, recs[1].Time(), cmpopts.EquateApproxTime(time.Second)); diff != "" {
		t.Fatalf("record[1] Time mismatch:\n%s", diff)
	}
}

func TestNew_EventBridgePassthrough(t *testing.T) {
	raw := readSample(t, "generic/eventbridge_unknown_source.json")
	ev, err := New(raw)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := ev.Source(); got != "com.example.custom" {
		t.Fatalf("Source: %q", got)
	}
	if got := ev.DetailType(); got != "Some Custom Event" {
		t.Fatalf("DetailType: %q", got)
	}
	if got := ev.Region(); got != "us-east-1" {
		t.Fatalf("Region: %q", got)
	}
	if got := ev.AccountID(); got != "123456789012" {
		t.Fatalf("AccountID: %q", got)
	}
}

func TestSource_StripsAWSPrefix(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "aws.ecs", want: "ecs"},
		{in: "aws.health", want: "health"},
		{in: "codepipeline", want: "codepipeline"},
		{in: "", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			payload := map[string]any{}
			if tc.in != "" {
				payload["source"] = tc.in
			}
			ev := mustEvent(t, payload)
			if got := ev.Source(); got != tc.want {
				t.Fatalf("Source(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRegion_FallbackChain(t *testing.T) {
	t.Run("explicit-region-wins", func(t *testing.T) {
		ev := mustEvent(t, map[string]any{
			"region":    "eu-west-2",
			"resources": []string{"arn:aws:ec2:us-east-1:123456789012:instance/i-0123"},
		})
		if got := ev.Region(); got != "eu-west-2" {
			t.Fatalf("Region: %q", got)
		}
	})
	t.Run("arn-region-fallback", func(t *testing.T) {
		ev := mustEvent(t, map[string]any{
			"resources": []string{"arn:aws:ec2:eu-central-1:123456789012:instance/i-0123"},
		})
		if got := ev.Region(); got != "eu-central-1" {
			t.Fatalf("Region: %q", got)
		}
	})
	t.Run("default-fallback", func(t *testing.T) {
		ev := mustEvent(t, map[string]any{})
		if got := ev.Region(); got != "us-east-1" {
			t.Fatalf("Region: %q", got)
		}
	})
}

func TestTime_Precedence(t *testing.T) {
	t.Run("message-time-wins", func(t *testing.T) {
		ev := mustEvent(t, map[string]any{
			"time": "2024-01-15T10:00:00Z",
		})
		if got := ev.Time(); !got.Equal(mustTime(t, "2024-01-15T10:00:00Z")) {
			t.Fatalf("Time: %v", got)
		}
	})
	t.Run("sns-timestamp-fallback", func(t *testing.T) {
		raw := readSample(t, "sns_envelope/single_record.json")
		ev, err := New(raw)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		want := mustTime(t, "2019-01-02T12:45:07Z")
		if diff := cmp.Diff(want, ev.Time(), cmpopts.EquateApproxTime(time.Second)); diff != "" {
			t.Fatalf("Time mismatch:\n%s", diff)
		}
	})
	t.Run("now-fallback", func(t *testing.T) {
		ev := mustEvent(t, map[string]any{})
		got := ev.Time()
		if time.Since(got) > 5*time.Second {
			t.Fatalf("Time fell back too far in the past: %v", got)
		}
	})
}

func TestGet_ReturnsRawJSON(t *testing.T) {
	ev := mustEvent(t, map[string]any{
		"detail": map[string]any{"nested": true},
		"region": "us-east-1",
	})
	if got := ev.Get(""); got != nil {
		t.Fatalf("Get(\"\") should be nil")
	}
	if got := ev.Get("missing"); got != nil {
		t.Fatalf("Get(missing) should be nil")
	}
	got := ev.Get("detail")
	if len(got) == 0 {
		t.Fatalf("Get(detail) returned empty")
	}
	var dest map[string]any
	if err := json.Unmarshal(got, &dest); err != nil {
		t.Fatalf("Get(detail) not valid JSON: %v", err)
	}
	if dest["nested"] != true {
		t.Fatalf("Get(detail).nested mismatch: %v", dest["nested"])
	}
}

func TestRaw_RoundTrip(t *testing.T) {
	payload := map[string]any{"hello": "world"}
	ev := mustEvent(t, payload)
	var dest map[string]any
	if err := json.Unmarshal(ev.Raw(), &dest); err != nil {
		t.Fatalf("Raw not valid JSON: %v", err)
	}
	if dest["hello"] != "world" {
		t.Fatalf("Raw round-trip mismatch: %v", dest)
	}
}

func TestRecords_PassesThroughForNonSNS(t *testing.T) {
	ev := mustEvent(t, map[string]any{"source": "aws.ecs"})
	recs := ev.Records()
	if len(recs) != 1 || recs[0] != ev {
		t.Fatalf("Records: expected [self], got %v", recs)
	}
}

func TestRecords_DegradesGracefullyOnMalformedRecordsField(t *testing.T) {
	// "Records" is present but not an array — fall back to single event.
	raw := json.RawMessage(`{"Records": {"oops": true}}`)
	ev, err := New(raw)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	recs := ev.Records()
	if len(recs) != 1 {
		t.Fatalf("Records: want 1, got %d", len(recs))
	}
}

func TestSNSMessage_InvalidJSON_FallsBackToString(t *testing.T) {
	raw := json.RawMessage(`{"Records":[{"EventSource":"aws:sns","Sns":{"Message":"{not really json","Subject":"S","Timestamp":"2024-06-01T12:00:00Z"}}]}`)
	ev, err := New(raw)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var s string
	if err := json.Unmarshal(ev.Message(), &s); err != nil {
		t.Fatalf("invalid-looking JSON should fall back to string: %v", err)
	}
	if s != "{not really json" {
		t.Fatalf("body mismatch: %q", s)
	}
}

func TestSNSMessage_EmptyMessage(t *testing.T) {
	raw := json.RawMessage(`{"Records":[{"EventSource":"aws:sns","EventSubscriptionArn":"arn:aws:sns:us-east-1:123:t","Sns":{"Message":"","Subject":"X","Timestamp":"2024-06-01T12:00:00Z"}}]}`)
	ev, err := New(raw)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Empty message: fall back to using the record itself.
	if len(ev.Message()) == 0 {
		t.Fatalf("Message should be non-empty")
	}
	if ev.Subject() != "X" {
		t.Fatalf("Subject: %q", ev.Subject())
	}
}

func TestSNSRecord_MalformedSnsBody_FallsBackToRecord(t *testing.T) {
	raw := json.RawMessage(`{"Records":[{"EventSource":"aws:sns","Sns":"not-an-object"}]}`)
	ev, err := New(raw)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(ev.Message()) == 0 {
		t.Fatalf("Message should fall back to record bytes")
	}
}

func TestAccountID_FromARNFallback(t *testing.T) {
	ev := mustEvent(t, map[string]any{
		"resources": []string{"arn:aws:rds:us-east-1:999988887777:db:my-db"},
	})
	if got := ev.AccountID(); got != "999988887777" {
		t.Fatalf("AccountID: %q", got)
	}
}

func TestSource_ARNFallback(t *testing.T) {
	ev := mustEvent(t, map[string]any{
		"arn": "arn:aws:cloudformation:us-east-1:123456789012:stack/my-stack",
	})
	if got := ev.Source(); got != "cloudformation" {
		t.Fatalf("Source: %q", got)
	}
}

func TestSubject_EmptyForEventBridge(t *testing.T) {
	ev := mustEvent(t, map[string]any{"source": "aws.ec2"})
	if got := ev.Subject(); got != "" {
		t.Fatalf("Subject: %q", got)
	}
}

func readSample(t *testing.T, rel string) json.RawMessage {
	t.Helper()
	path := filepath.Join(samplesRoot, rel)
	b, err := os.ReadFile(path) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("read sample %s: %v", path, err)
	}
	return b
}

func mustEvent(t *testing.T, payload map[string]any) *Event {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ev, err := New(raw)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return ev
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return tt.UTC()
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0) }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
