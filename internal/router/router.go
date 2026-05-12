package router

import (
	"context"
	"fmt"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

// Router owns the ordered list of parsers and dispatches each event to the
// first one whose Match returns true. The "generic" parser is registered
// last so it acts as a catch-all (asserted by the handler wiring).
//
// The router walks the entire matching set in order: if a matched parser
// returns a "silenced" result (nil message + nil error), control falls
// through to the next matched parser. A parser returning an error is
// collected but does not halt the walk, so a later parser can still
// produce a message.
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

// Route walks the registered parsers and returns the message produced by
// the first match that emits a non-nil result. Returns (nil, nil) when no
// parser matches or every match was silenced.
//
// Errors from individual parser.Parse calls are collected and returned as
// a wrapped error only if no successful message was produced — successful
// downstream sends always take precedence.
func (r *Router) Route(ctx context.Context, e *envelope.Event) (*slack.Message, error) {
	var lastErr error
	for _, p := range r.parsers {
		if !p.Match(e) {
			continue
		}
		msg, err := p.Parse(ctx, e)
		if err != nil {
			lastErr = fmt.Errorf("parser %s: %w", p.Name(), err)
			continue
		}
		if msg != nil {
			return msg, nil
		}
	}
	return nil, lastErr
}
