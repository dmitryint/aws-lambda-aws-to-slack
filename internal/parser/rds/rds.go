// Package rds renders Slack messages for Amazon RDS event-subscription SNS
// notifications. Matches when the inner message contains
// `"Event Source": "db-instance"`.
package rds

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/slack"
)

const (
	name           = "rds"
	dbInstanceKind = "db-instance"
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

// Parse renders the Slack message for an RDS event-subscription notification.
func (Parser) Parse(_ context.Context, e *envelope.Event) (*slack.Message, error) {
	m, ok := decode(e)
	if !ok {
		return nil, fmt.Errorf("rds: payload is not a JSON object")
	}

	title := slack.Link(m.IdentifierLink, m.SourceID)
	author := "Amazon RDS"

	blocks := []slack.Block{
		slack.SectionBlock(fmt.Sprintf("*%s*\n_%s_", title, author)),
		slack.SectionBlock(m.EventMessage),
	}

	fallback := fmt.Sprintf("%s: %s", m.SourceID, m.EventMessage)
	return slack.NewMessage(slack.ColorAccent, fallback, blocks...), nil
}
