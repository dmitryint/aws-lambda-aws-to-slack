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
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser"
	genericparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/generic"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/router"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

// recordingPoster captures every Post call. The optional errFunc returns
// an error for the call at the given (1-based) index so partial-failure
// tests can target a specific record.
type recordingPoster struct {
	mu       sync.Mutex
	posted   []*slack.Message
	inFlight atomic.Int32
	maxSeen  atomic.Int32
	delay    time.Duration
	errFunc  func(callIndex int) error
}

func (r *recordingPoster) Post(ctx context.Context, m *slack.Message) error {
	r.mu.Lock()
	r.posted = append(r.posted, m)
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
// record produces a non-nil message — exactly what we need to count
// Slack posts.
func testRouter() *router.Router {
	r := router.New()
	r.Register(genericparser.New())
	return r
}

func newTestHandler(t *testing.T, poster SlackPoster) *Handler {
	t.Helper()
	cfg := &config.Config{SlackHookURL: "http://invalid.example/never-called"}
	return New(cfg, aws.Config{}, WithRouter(testRouter()), WithSlackPoster(poster))
}

func TestHandle_MultiRecordFanOut(t *testing.T) {
	raw := readSample(t, "multi_record.json")
	poster := &recordingPoster{}
	h := newTestHandler(t, poster)

	if err := h.Handle(t.Context(), raw); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(poster.posted) != 2 {
		t.Fatalf("posted %d messages, want 2", len(poster.posted))
	}
}

func TestHandle_PartialFailure_StillPostsOthers(t *testing.T) {
	raw := readSample(t, "multi_record.json")
	postErr := errors.New("slack outage")
	poster := &recordingPoster{
		errFunc: func(i int) error {
			if i == 2 {
				return postErr
			}
			return nil
		},
	}
	h := newTestHandler(t, poster)

	err := h.Handle(t.Context(), raw)
	if err == nil {
		t.Fatal("expected handler error from partial failure")
	}
	if len(poster.posted) != 2 {
		t.Fatalf("posted %d messages, want 2 (partial failure must still attempt all)", len(poster.posted))
	}
	if !errors.Is(err, postErr) {
		t.Fatalf("error chain missing post error: %v", err)
	}
}

func TestHandle_BoundedConcurrency(t *testing.T) {
	const totalRecords = 10
	raw := buildSyntheticSNSEnvelope(t, totalRecords)
	poster := &recordingPoster{delay: 20 * time.Millisecond}
	h := newTestHandler(t, poster)

	if err := h.Handle(t.Context(), raw); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if got := poster.maxSeen.Load(); got > int32(maxInFlight) {
		t.Fatalf("max concurrent Posts = %d, exceeds bound %d", got, maxInFlight)
	}
	if got := poster.maxSeen.Load(); got < 2 {
		// On a single-CPU runner this could legitimately serialize to 1;
		// still useful as a sanity check the goroutines actually overlap.
		if runtime.NumCPU() > 1 {
			t.Fatalf("max concurrent Posts = %d, expected fan-out to overlap on a multi-CPU host", got)
		}
	}
	if len(poster.posted) != totalRecords {
		t.Fatalf("posted %d messages, want %d", len(poster.posted), totalRecords)
	}
}

func TestHandle_EmptyRecords(t *testing.T) {
	raw := json.RawMessage(`{"Records":[]}`)
	poster := &recordingPoster{}
	h := newTestHandler(t, poster)

	if err := h.Handle(t.Context(), raw); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(poster.posted) != 0 {
		t.Fatalf("posted %d messages on empty envelope", len(poster.posted))
	}
}

func TestHandle_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`{"not closed":`)
	poster := &recordingPoster{}
	h := newTestHandler(t, poster)

	err := h.Handle(t.Context(), raw)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "envelope") {
		t.Fatalf("error %q should mention envelope parsing", err)
	}
	if len(poster.posted) != 0 {
		t.Fatalf("posted on invalid JSON: %d", len(poster.posted))
	}
}

// TestHandle_Slack429ThenSuccess wires the real slack.Client with a
// server that 429s once then succeeds, exercising the retry contract.
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
	cfg := &config.Config{SlackHookURL: srv.URL}
	h := New(cfg, aws.Config{}, WithRouter(testRouter()), WithSlackPoster(client))

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
	poster := &recordingPoster{}
	h := newTestHandler(t, poster)

	if err := h.Handle(t.Context(), raw); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(poster.posted) != 1 {
		t.Fatalf("posted %d, want 1 for direct payload", len(poster.posted))
	}
}

// TestHandle_RouterError_PropagatesAsHandlerError ensures parser errors
// surface back to the Lambda runtime (so the Errors metric increments).
func TestHandle_RouterError_PropagatesAsHandlerError(t *testing.T) {
	errParser := &erroringParser{err: errors.New("boom from parser")}
	r := router.New()
	r.Register(errParser)
	cfg := &config.Config{SlackHookURL: "http://invalid.example"}
	poster := &recordingPoster{}
	h := New(cfg, aws.Config{}, WithRouter(r), WithSlackPoster(poster))

	raw := readSample(t, "single_record.json")
	err := h.Handle(t.Context(), raw)
	if err == nil {
		t.Fatal("expected handler error from router")
	}
	if !errors.Is(err, errParser.err) {
		t.Fatalf("error chain missing parser error: %v", err)
	}
	if len(poster.posted) != 0 {
		t.Fatalf("posted on router error: %d", len(poster.posted))
	}
}

// TestNew_DefaultsCarryThrough confirms the production constructor wires
// a non-nil router and a slack client. We don't invoke Handle in this
// test — that would attempt a real HTTP POST to the configured URL.
func TestNew_DefaultsCarryThrough(t *testing.T) {
	cfg := &config.Config{SlackHookURL: "http://invalid.example"}
	h := New(cfg, aws.Config{})
	if h.router == nil {
		t.Fatal("router is nil")
	}
	if h.slack == nil {
		t.Fatal("slack is nil")
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
func (p erroringParser) Parse(context.Context, *envelope.Event) (*slack.Message, error) {
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
