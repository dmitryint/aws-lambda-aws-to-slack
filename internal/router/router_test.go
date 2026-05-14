package router

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

// fakeParser is the test seam that lets us drive the router state machine
// without booting any real parser.
type fakeParser struct {
	name    string
	matches bool
	msg     *slack.Message
	err     error
	calls   int
}

func (f *fakeParser) Name() string                 { return f.name }
func (f *fakeParser) Match(_ *envelope.Event) bool { return f.matches }
func (f *fakeParser) Parse(_ context.Context, _ *envelope.Event) (*slack.Message, error) {
	f.calls++
	return f.msg, f.err
}

func newEvent(t *testing.T) *envelope.Event {
	t.Helper()
	ev, err := envelope.New(json.RawMessage(`{"source":"aws.test"}`))
	if err != nil {
		t.Fatalf("envelope.New: %v", err)
	}
	return ev
}

func TestRoute_SingleMatchReturnsMessage(t *testing.T) {
	want := slack.NewMessage(slack.ColorOK, "ok")
	p := &fakeParser{name: "p", matches: true, msg: want}
	r := New()
	r.Register(p)

	got, err := r.Route(t.Context(), newEvent(t))
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got != want {
		t.Fatalf("Route returned different message: %+v", got)
	}
	if p.calls != 1 {
		t.Fatalf("parser called %d times, want 1", p.calls)
	}
}

func TestRoute_SilencedParserIsTerminal(t *testing.T) {
	// A matches and returns (nil, nil) — "matched but silenced". The router
	// must stop walking; B (a catch-all that would otherwise render) is
	// never consulted.
	a := &fakeParser{name: "a", matches: true, msg: nil}
	b := &fakeParser{name: "b", matches: true, msg: slack.NewMessage(slack.ColorOK, "from-b")}
	r := New()
	r.Register(a)
	r.Register(b)

	got, err := r.Route(t.Context(), newEvent(t))
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil message (silenced), got %+v", got)
	}
	if a.calls != 1 {
		t.Fatalf("parser a should be called once, got %d", a.calls)
	}
	if b.calls != 0 {
		t.Fatalf("parser b must not be called after a silences the event, got %d calls", b.calls)
	}
}

func TestRoute_NonMatchingParsersSkipped(t *testing.T) {
	a := &fakeParser{name: "a", matches: false}
	b := &fakeParser{name: "b", matches: true, msg: slack.NewMessage(slack.ColorOK, "from-b")}
	r := New()
	r.Register(a)
	r.Register(b)

	got, err := r.Route(t.Context(), newEvent(t))
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got == nil {
		t.Fatal("expected message from b")
	}
	if a.calls != 0 {
		t.Fatalf("non-matching parser a was called %d times", a.calls)
	}
}

func TestRoute_NoMatchReturnsNilNil(t *testing.T) {
	a := &fakeParser{name: "a", matches: false}
	r := New()
	r.Register(a)

	got, err := r.Route(t.Context(), newEvent(t))
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil message, got %+v", got)
	}
}

func TestRoute_ParserErrorRecordedAndWalkContinues(t *testing.T) {
	wantErr := errors.New("boom")
	a := &fakeParser{name: "a", matches: true, err: wantErr}
	b := &fakeParser{name: "b", matches: true, msg: slack.NewMessage(slack.ColorOK, "from-b")}
	r := New()
	r.Register(a)
	r.Register(b)

	// b succeeds → a's error is suppressed.
	got, err := r.Route(t.Context(), newEvent(t))
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got == nil || got.Attachments[0].Fallback != "from-b" {
		t.Fatalf("walk did not fall through to b: %+v", got)
	}
}

func TestRoute_AllErrorsAndNoMessageSurfacesLastErr(t *testing.T) {
	wantErr := errors.New("boom")
	a := &fakeParser{name: "a", matches: true, err: wantErr}
	r := New()
	r.Register(a)

	got, err := r.Route(t.Context(), newEvent(t))
	if got != nil {
		t.Fatalf("expected nil message")
	}
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped wantErr, got %v", err)
	}
}

func TestParsers_ExposesRegistrationOrder(t *testing.T) {
	a := &fakeParser{name: "a"}
	b := &fakeParser{name: "b"}
	r := New()
	r.Register(a)
	r.Register(b)

	got := r.Parsers()
	if len(got) != 2 || got[0].Name() != "a" || got[1].Name() != "b" {
		t.Fatalf("Parsers() order wrong: %v", got)
	}
}
