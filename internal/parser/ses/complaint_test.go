package ses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
)

const (
	complaintSamplesRoot = "../../../samples/ses/complaint"
	complaintGoldenRoot  = "testdata/golden/complaint"
)

func TestSESComplaint_Name(t *testing.T) {
	if got := NewComplaint().Name(); got != "ses-complaint" {
		t.Fatalf("Name = %q, want %q", got, "ses-complaint")
	}
}

func TestSESComplaint_Match(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "right-type", raw: `{"notificationType":"Complaint"}`, want: true},
		{name: "bounce-not-complaint", raw: `{"notificationType":"Bounce"}`, want: false},
		{name: "received-not-complaint", raw: `{"notificationType":"Received"}`, want: false},
		{name: "no-type", raw: `{}`, want: false},
		{name: "non-object", raw: `"hello"`, want: false},
	}
	p := NewComplaint()
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

func TestSESComplaint_Parse_ErrorOnNonObject(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`"plain"`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := NewComplaint().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error on non-object payload")
	}
}

func TestSESComplaint_RenderRecipients(t *testing.T) {
	if got := renderComplainedRecipients(nil); got != "" {
		t.Fatalf("empty list should produce empty string, got %q", got)
	}
	got := renderComplainedRecipients([]complainedRecipient{
		{EmailAddress: "a@example.com"},
		{EmailAddress: "b@example.com"},
	})
	want := "a@example.com\nb@example.com"
	if got != want {
		t.Fatalf("render = %q, want %q", got, want)
	}
}

func TestSESComplaint_SampleGoldens(t *testing.T) {
	runSESGoldenSuite(t, NewComplaint(), complaintSamplesRoot, complaintGoldenRoot)
}
