// Package transport defines the Renderer contract every notification
// transport implements. v1 wires only the Slack renderer at
// internal/transport/slack; future renderers (SES email digest,
// PagerDuty, etc.) plug into the same interface without touching parsers.
package transport

import (
	"context"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

// Renderer translates a notify.Notification into the wire format of one
// downstream service and sends it. The handler iterates a slice of
// Renderers per record and skips those whose Accepts predicate rejects
// the Notification's Severity.
type Renderer interface {
	// Name identifies the renderer in logs and metrics.
	Name() string

	// Accepts returns true when the renderer should receive a Notification
	// of the given Severity. Implementations typically hold a configurable
	// minimum-severity floor so the same Notification can be routed to
	// noisy and noise-averse channels from one handler.
	Accepts(notify.Severity) bool

	// Send delivers the Notification. Returns nil on success; the handler
	// aggregates per-renderer errors so one failing transport never
	// suppresses the others.
	Send(ctx context.Context, n *notify.Notification) error
}
