package parser

import (
	"context"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

// Parser is the contract every per-source parser implements.
//
// Match is a cheap predicate (no network, no allocation beyond what the
// envelope already holds); the router walks the registered parsers in
// order and dispatches the first one that returns true.
type Parser interface {
	// Name returns a stable identifier used in logs and the router test
	// that asserts ordering invariants.
	Name() string

	// Match returns true when this parser owns the given event.
	Match(e *envelope.Event) bool

	// Parse renders the Slack message for the given event. A nil message
	// with a nil error means the parser deliberately silenced the event
	// (matched but no alert needed).
	Parse(ctx context.Context, e *envelope.Event) (*slack.Message, error)
}
