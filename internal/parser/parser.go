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

	// Match returns true when this parser owns the given event. A true
	// return is the parser's claim — the router stops at the first match
	// and does not consult later parsers. Filters that should defer to a
	// downstream parser (typically the generic catch-all) belong in Match,
	// not Parse.
	Match(e *envelope.Event) bool

	// Parse renders the Slack message for the given event. The result is
	// terminal: a nil message with a nil error means the parser
	// deliberately silenced the event (matched but no alert is posted),
	// and the router will not fall through to another parser.
	Parse(ctx context.Context, e *envelope.Event) (*slack.Message, error)
}
