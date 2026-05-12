package ses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
)

const (
	receivedSamplesRoot = "../../../samples/ses/received"
	receivedGoldenRoot  = "testdata/golden/received"
)

func TestSESReceived_Name(t *testing.T) {
	if got := NewReceived().Name(); got != "ses-received" {
		t.Fatalf("Name = %q, want %q", got, "ses-received")
	}
}

func TestSESReceived_Match(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "right-type", raw: `{"notificationType":"Received"}`, want: true},
		{name: "bounce-not-received", raw: `{"notificationType":"Bounce"}`, want: false},
		{name: "complaint-not-received", raw: `{"notificationType":"Complaint"}`, want: false},
		{name: "no-type", raw: `{}`, want: false},
		{name: "non-object", raw: `"hello"`, want: false},
	}
	p := NewReceived()
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

func TestSESReceived_Parse_ErrorOnNonObject(t *testing.T) {
	ev, err := envelope.New(json.RawMessage(`"plain"`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	if _, err := NewReceived().Parse(context.Background(), ev); err == nil {
		t.Fatal("Parse should error on non-object payload")
	}
}

func TestSESReceived_HasAttachments(t *testing.T) {
	cases := []struct {
		name    string
		headers []mailHeader
		content string
		want    bool
	}{
		{
			name: "plain-text",
			headers: []mailHeader{
				{Name: "Content-Type", Value: "text/plain; charset=UTF-8"},
			},
			content: "Hello",
			want:    false,
		},
		{
			name: "multipart-no-attachment",
			headers: []mailHeader{
				{Name: "Content-Type", Value: "multipart/alternative; boundary=B"},
			},
			content: "no attachment marker",
			want:    false,
		},
		{
			name: "multipart-with-attachment",
			headers: []mailHeader{
				{Name: "Content-Type", Value: "multipart/mixed; boundary=B"},
			},
			content: "...\nContent-Disposition: attachment; filename=x.pdf\n...",
			want:    true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasAttachments(tc.headers, tc.content); got != tc.want {
				t.Fatalf("hasAttachments = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSESReceived_SampleGoldens(t *testing.T) {
	runSESGoldenSuite(t, NewReceived(), receivedSamplesRoot, receivedGoldenRoot)
}
