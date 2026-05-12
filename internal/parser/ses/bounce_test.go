package ses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
)

const (
	bounceSamplesRoot = "../../../samples/ses/bounce"
	bounceGoldenRoot  = "testdata/golden/bounce"
)

func TestSESBounce_Name(t *testing.T) {
	if got := NewBounce().Name(); got != "ses-bounce" {
		t.Fatalf("Name = %q, want %q", got, "ses-bounce")
	}
}

func TestSESBounce_Match(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "right-type", raw: `{"notificationType":"Bounce"}`, want: true},
		{name: "complaint-not-bounce", raw: `{"notificationType":"Complaint"}`, want: false},
		{name: "received-not-bounce", raw: `{"notificationType":"Received"}`, want: false},
		{name: "no-type", raw: `{}`, want: false},
		{name: "non-object", raw: `"hello"`, want: false},
	}
	p := NewBounce()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev, err := envelope.New(json.RawMessage(tc.raw))
			if err != nil {
				t.Fatalf("envelope.New: %v", err)
			}
			if got := p.Match(ev); got != tc.want {
				t.Fatalf("Match = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSESBounce_Parse_ErrorOnNonObject(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`"plain"`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := NewBounce().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error on non-object payload")
	}
}

func TestSESBounce_BounceColor(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Permanent", "danger"},
		{"Transient", "#439FE0"},
		{"Undetermined", "#dddddd"},
		{"", "#dddddd"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := bounceColor(tc.in); got != tc.want {
				t.Fatalf("bounceColor(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSESBounce_RenderRecipients(t *testing.T) {
	if got := renderBouncedRecipients(nil); got != "" {
		t.Fatalf("empty list should produce empty string, got %q", got)
	}
	got := renderBouncedRecipients([]bouncedRecipient{
		{EmailAddress: "a@example.com", Action: "failed", Status: "5.1.1", DiagnosticCode: "smtp; 550"},
		{EmailAddress: "b@example.com"},
	})
	want := "a@example.com — failed; 5.1.1; smtp; 550\nb@example.com"
	if got != want {
		t.Fatalf("render = %q, want %q", got, want)
	}
}

func TestSESBounce_SampleGoldens(t *testing.T) {
	runSESGoldenSuite(t, NewBounce(), bounceSamplesRoot, bounceGoldenRoot)
}
