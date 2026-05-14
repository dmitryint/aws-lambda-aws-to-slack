package router

import (
	"context"
	"fmt"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser"
)

// Router owns the ordered list of parsers and dispatches each event to the
// first one whose Match returns true. The "generic" parser is registered
// last so it acts as a catch-all (asserted by the handler wiring).
//
// Match is the claim: the first parser whose Match returns true owns the
// event, and its Parse result is terminal — regardless of whether Parse
// returns a message or `(nil, nil)` (deliberate silence). A parser whose
// Parse returns an error is the exception: the error is collected and the
// walk continues so a later parser can still produce a message.
type Router struct {
	parsers []parser.Parser
}

// New returns an empty Router. Callers register parsers in the desired
// waterfall order; ordering is part of the contract.
func New() *Router {
	return &Router{}
}

// Register appends a parser to the waterfall.
func (r *Router) Register(p parser.Parser) {
	r.parsers = append(r.parsers, p)
}

// Parsers returns the registered parsers in order. Exported for tests that
// assert the waterfall contract (e.g. "generic is always last").
func (r *Router) Parsers() []parser.Parser {
	out := make([]parser.Parser, len(r.parsers))
	copy(out, r.parsers)
	return out
}

// Route walks the registered parsers and stops at the first match. The
// matched parser's Parse result is final: a non-nil Notification is
// returned for delivery; (nil, nil) silences the event entirely. The
// only exception is an error from Parse — the walk continues so a later
// parser can still produce a Notification, and the last error is returned
// only if no parser succeeded.
func (r *Router) Route(ctx context.Context, e *envelope.Event) (*notify.Notification, error) {
	var lastErr error
	for _, p := range r.parsers {
		if !p.Match(e) {
			continue
		}
		n, err := p.Parse(ctx, e)
		if err != nil {
			lastErr = fmt.Errorf("parser %s: %w", p.Name(), err)
			continue
		}
		return n, nil
	}
	return nil, lastErr
}
