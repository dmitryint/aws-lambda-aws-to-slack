// Package rds renders Amazon RDS event-subscription SNS notifications into
// the transport-neutral notify.Notification shape. Matches when the inner
// message contains `"Event Source": "db-instance"`.
package rds

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/notify"
)

const (
	name           = "rds"
	dbInstanceKind = "db-instance"

	subtitleAmazonRDS = "Amazon RDS"
)

// Parser handles RDS event-subscription SNS notifications.
type Parser struct{}

// New returns a Parser ready to register with the router.
func New() *Parser { return &Parser{} }

// Name returns the stable parser identifier.
func (Parser) Name() string { return name }

// message is the subset of the RDS event payload the parser reads.
type message struct {
	EventSource    string `json:"Event Source"`
	EventMessage   string `json:"Event Message"`
	SourceID       string `json:"Source ID"`
	IdentifierLink string `json:"Identifier Link"`
}

// decode pulls the typed payload from the envelope when it is a JSON object.
func decode(e *envelope.Event) (message, bool) {
	raw := e.Message()
	if len(raw) == 0 || raw[0] != '{' {
		return message{}, false
	}
	var m message
	if err := json.Unmarshal(raw, &m); err != nil {
		return message{}, false
	}
	return m, true
}

// Match returns true when the SNS message reports `Event Source: db-instance`.
func (Parser) Match(e *envelope.Event) bool {
	m, ok := decode(e)
	return ok && m.EventSource == dbInstanceKind
}

// Parse renders the Notification for an RDS event-subscription notification.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*notify.Notification, error) {
	m, ok := decode(e)
	if !ok {
		return nil, fmt.Errorf("rds: payload is not a JSON object")
	}

	return &notify.Notification{
		Source:   name,
		Severity: severityFor(m.EventMessage),
		Title:    m.SourceID,
		TitleURL: m.IdentifierLink,
		Subtitle: subtitleAmazonRDS,
		Summary:  m.EventMessage,
		Fallback: fmt.Sprintf("%s: %s", m.SourceID, m.EventMessage),
	}, nil
}

// severityFor classifies the RDS event message into a Severity per the
// spec: failover / fatal → Critical, maintenance → Notice,
// backup / availability → OK; anything else defaults to Notice.
func severityFor(msg string) notify.Severity {
	lower := strings.ToLower(msg)
	switch {
	case containsAny(lower, "failover", "fatal", "failed", "error"):
		return notify.SeverityCritical
	case containsAny(lower, "maintenance", "patch", "upgrade", "reboot", "applying"):
		return notify.SeverityNotice
	case containsAny(lower, "backup", "snapshot", "available", "restored", "created"):
		return notify.SeverityOK
	default:
		return notify.SeverityNotice
	}
}

// containsAny reports whether s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
