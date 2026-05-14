package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/config"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser"
	genericparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/generic"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/router"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/transport"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/transport/slack"
)

// recordingRenderer captures every Notification handed to Send. The optional
// errFunc returns an error for the call at the given (1-based) index so
// partial-failure tests can target a specific record.
type recordingRenderer struct {
	mu       sync.Mutex
	posted   []*notify.Notification
	inFlight atomic.Int32
	maxSeen  atomic.Int32
	delay    time.Duration
	errFunc  func(callIndex int) error
}

// Name identifies the recording renderer in handler logs.
func (*recordingRenderer) Name() string { return "recording" }

// Accepts always returns true — the recording renderer captures every
// notification regardless of severity.
func (*recordingRenderer) Accepts(notify.Severity) bool { return true }

// Send records the Notification and exercises the bounded-concurrency seam.
func (r *recordingRenderer) Send(ctx context.Context, n *notify.Notification) error {
	r.mu.Lock()
	r.posted = append(r.posted, n)
	idx := len(r.posted)
	r.mu.Unlock()

	current := r.inFlight.Add(1)
	defer r.inFlight.Add(-1)
	for {
		seen := r.maxSeen.Load()
		if current <= seen {
			break
		}
		if r.maxSeen.CompareAndSwap(seen, current) {
			break
		}
	}

	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if r.errFunc != nil {
		if err := r.errFunc(idx); err != nil {
			return err
		}
	}
	return nil
}

// ensure recordingRenderer satisfies the transport.Renderer interface.
var _ transport.Renderer = (*recordingRenderer)(nil)

func readSample(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "samples", "sns_envelope", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sample %s: %v", name, err)
	}
	return raw
}

// testRouter returns a router with only the generic parser, so every
// record produces a non-nil notification — exactly what we need to count
// deliveries.
func testRouter() *router.Router {
	r := router.New()
	r.Register(genericparser.New())
	return r
}

func newTestHandler(t *testing.T, rec *recordingRenderer) *Handler {
	t.Helper()
	cfg := &config.Config{SlackHookURL: "http://invalid.example/never-called"}
	return New(cfg, aws.Config{}, WithRouter(testRouter()), WithRenderers(rec))
}

func TestHandle_MultiRecordFanOut(t *testing.T) {
	raw := readSample(t, "multi_record.json")
	rec := &recordingRenderer{}
	h := newTestHandler(t, rec)

	if err := h.Handle(t.Context(), raw); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(rec.posted) != 2 {
		t.Fatalf("posted %d notifications, want 2", len(rec.posted))
	}
}

func TestHandle_PartialFailure_StillPostsOthers(t *testing.T) {
	raw := readSample(t, "multi_record.json")
	postErr := errors.New("slack outage")
	rec := &recordingRenderer{
		errFunc: func(i int) error {
			if i == 2 {
				return postErr
			}
			return nil
		},
	}
	h := newTestHandler(t, rec)

	err := h.Handle(t.Context(), raw)
	if err == nil {
		t.Fatal("expected handler error from partial failure")
	}
	if len(rec.posted) != 2 {
		t.Fatalf("posted %d notifications, want 2 (partial failure must still attempt all)", len(rec.posted))
	}
	if !errors.Is(err, postErr) {
		t.Fatalf("error chain missing post error: %v", err)
	}
}

func TestHandle_BoundedConcurrency(t *testing.T) {
	const totalRecords = 10
	raw := buildSyntheticSNSEnvelope(t, totalRecords)
	rec := &recordingRenderer{delay: 20 * time.Millisecond}
	h := newTestHandler(t, rec)

	if err := h.Handle(t.Context(), raw); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if got := rec.maxSeen.Load(); got > int32(maxInFlight) {
		t.Fatalf("max concurrent sends = %d, exceeds bound %d", got, maxInFlight)
	}
	if got := rec.maxSeen.Load(); got < 2 {
		// On a single-CPU runner this could legitimately serialize to 1;
		// still useful as a sanity check the goroutines actually overlap.
		if runtime.NumCPU() > 1 {
			t.Fatalf("max concurrent sends = %d, expected fan-out to overlap on a multi-CPU host", got)
		}
	}
	if len(rec.posted) != totalRecords {
		t.Fatalf("posted %d notifications, want %d", len(rec.posted), totalRecords)
	}
}

func TestHandle_EmptyRecords(t *testing.T) {
	raw := json.RawMessage(`{"Records":[]}`)
	rec := &recordingRenderer{}
	h := newTestHandler(t, rec)

	if err := h.Handle(t.Context(), raw); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(rec.posted) != 0 {
		t.Fatalf("posted %d notifications on empty envelope", len(rec.posted))
	}
}

func TestHandle_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`{"not closed":`)
	rec := &recordingRenderer{}
	h := newTestHandler(t, rec)

	err := h.Handle(t.Context(), raw)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "envelope") {
		t.Fatalf("error %q should mention envelope parsing", err)
	}
	if len(rec.posted) != 0 {
		t.Fatalf("posted on invalid JSON: %d", len(rec.posted))
	}
}

// TestHandle_Slack429ThenSuccess wires the real slack.Renderer with a server
// that 429s once then succeeds, exercising the retry contract end-to-end.
func TestHandle_Slack429ThenSuccess(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := slack.New(srv.URL,
		slack.WithBaseBackoff(time.Millisecond),
		slack.WithSleep(func(time.Duration) {}),
	)
	renderer := slack.NewRenderer(client)
	cfg := &config.Config{SlackHookURL: srv.URL}
	h := New(cfg, aws.Config{}, WithRouter(testRouter()), WithRenderers(renderer))

	raw := readSample(t, "single_record.json")
	if err := h.Handle(t.Context(), raw); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("Slack call count = %d, want 2 (one 429 + one 200)", got)
	}
}

// TestHandle_DirectInvocationPayload covers the EventBridge / direct
// (non-SNS) shape: no Records[] wrapper, one event in the slice.
func TestHandle_DirectInvocationPayload(t *testing.T) {
	raw := json.RawMessage(`{"detail-type":"direct","source":"unit-test","detail":{"hello":"world"}}`)
	rec := &recordingRenderer{}
	h := newTestHandler(t, rec)

	if err := h.Handle(t.Context(), raw); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(rec.posted) != 1 {
		t.Fatalf("posted %d, want 1 for direct payload", len(rec.posted))
	}
}

// TestHandle_RouterError_PropagatesAsHandlerError ensures parser errors
// surface back to the Lambda runtime (so the Errors metric increments).
func TestHandle_RouterError_PropagatesAsHandlerError(t *testing.T) {
	errParser := &erroringParser{err: errors.New("boom from parser")}
	r := router.New()
	r.Register(errParser)
	cfg := &config.Config{SlackHookURL: "http://invalid.example"}
	rec := &recordingRenderer{}
	h := New(cfg, aws.Config{}, WithRouter(r), WithRenderers(rec))

	raw := readSample(t, "single_record.json")
	err := h.Handle(t.Context(), raw)
	if err == nil {
		t.Fatal("expected handler error from router")
	}
	if !errors.Is(err, errParser.err) {
		t.Fatalf("error chain missing parser error: %v", err)
	}
	if len(rec.posted) != 0 {
		t.Fatalf("posted on router error: %d", len(rec.posted))
	}
}

// TestNew_DefaultsCarryThrough confirms the production constructor wires
// a non-nil router and the default Slack renderer. We don't invoke Handle
// in this test — that would attempt a real HTTP POST to the configured URL.
func TestNew_DefaultsCarryThrough(t *testing.T) {
	cfg := &config.Config{SlackHookURL: "http://invalid.example"}
	h := New(cfg, aws.Config{})
	if h.router == nil {
		t.Fatal("router is nil")
	}
	if len(h.renderers) == 0 {
		t.Fatal("renderers is empty")
	}
	if h.log == nil {
		t.Fatal("logger is nil")
	}
	if len(h.router.Parsers()) == 0 {
		t.Fatal("default router has no parsers")
	}
}

// erroringParser is a test parser that always matches and returns the
// pre-programmed error.
type erroringParser struct{ err error }

func (erroringParser) Name() string               { return "erroring" }
func (erroringParser) Match(*envelope.Event) bool { return true }
func (p erroringParser) Parse(context.Context, *envelope.Event) (*notify.Notification, error) {
	return nil, p.err
}

// compile-time assertion: erroringParser is a parser.Parser.
var _ parser.Parser = erroringParser{}

// buildSyntheticSNSEnvelope synthesizes a multi-record SNS envelope with
// the given record count for the bounded-concurrency test.
func buildSyntheticSNSEnvelope(t *testing.T, n int) json.RawMessage {
	t.Helper()
	type snsBody struct {
		Message   string `json:"Message"`
		Subject   string `json:"Subject"`
		Timestamp string `json:"Timestamp"`
	}
	type record struct {
		EventSource          string  `json:"EventSource"`
		EventSubscriptionArn string  `json:"EventSubscriptionArn"`
		Sns                  snsBody `json:"Sns"`
	}
	type outer struct {
		Records []record `json:"Records"`
	}
	o := outer{Records: make([]record, 0, n)}
	for i := 0; i < n; i++ {
		o.Records = append(o.Records, record{
			EventSource:          "aws:sns",
			EventSubscriptionArn: "arn:aws:sns:us-east-1:123456789012:test:abcd",
			Sns: snsBody{
				Message:   fmt.Sprintf("synthetic record %d", i),
				Subject:   fmt.Sprintf("synthetic-%d", i),
				Timestamp: "2025-01-15T12:00:00.000Z",
			},
		})
	}
	raw, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("marshal synthetic envelope: %v", err)
	}
	return raw
}

// pointer assertion used to compile-time verify that erroringParser pointer
// receivers also satisfy the parser.Parser interface.
var _ parser.Parser = (*erroringParser)(nil)
