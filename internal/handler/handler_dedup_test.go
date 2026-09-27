package handler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/config"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/dedup"
	inspector2parser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/inspector2"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/router"
)

type memoryDedup struct {
	mu   sync.Mutex
	keys map[string]struct{}
}

func (m *memoryDedup) TryReserve(_ context.Context, key string, _ map[string]string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.keys == nil {
		m.keys = map[string]struct{}{}
	}
	if _, ok := m.keys[key]; ok {
		return false, nil
	}
	m.keys[key] = struct{}{}
	return true, nil
}

var _ dedup.Deduplicator = (*memoryDedup)(nil)

func newInspector2DedupHandler(t *testing.T, rec *recordingRenderer) (h *Handler, raw []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "samples", "inspector2", "finding_high_lambda.json"))
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	r := router.New()
	r.Register(inspector2parser.NewWithDedup(&memoryDedup{}))
	cfg := &config.Config{SlackHookURL: "http://invalid.example/never-called"}
	return New(cfg, aws.Config{}, WithRouter(r), WithRenderers(rec)), raw
}

func TestHandle_Inspector2_RetryAfterFailedDeliveryIsDelivered(t *testing.T) {
	rec := &recordingRenderer{errFunc: func(callIndex int) error {
		if callIndex == 1 {
			return errors.New("slack: exhausted 3 attempts: status 429")
		}
		return nil
	}}
	h, raw := newInspector2DedupHandler(t, rec)

	if err := h.Handle(t.Context(), raw); err == nil {
		t.Fatal("first invocation: Handle returned nil although delivery failed")
	}
	if err := h.Handle(t.Context(), raw); err != nil {
		t.Fatalf("retry invocation: Handle: %v", err)
	}
	if got := len(rec.posted); got != 2 {
		t.Fatalf("retry after a failed delivery reached the transport %d time(s) in total, want 2: the finding was silenced as a duplicate and never delivered", got)
	}
}

func TestHandle_Inspector2_RetryAfterSuccessfulDeliveryIsSilenced(t *testing.T) {
	rec := &recordingRenderer{}
	h, raw := newInspector2DedupHandler(t, rec)

	for i := range 2 {
		if err := h.Handle(t.Context(), raw); err != nil {
			t.Fatalf("invocation %d: Handle: %v", i+1, err)
		}
	}
	if got := len(rec.posted); got != 1 {
		t.Fatalf("finding delivered %d times, want 1", got)
	}
}
