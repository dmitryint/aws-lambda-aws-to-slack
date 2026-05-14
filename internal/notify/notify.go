// Package notify defines the transport-neutral message a parser emits.
//
// Parsers translate AWS events into a Notification carrying explicit
// Severity plus structured fields. Transports (Slack, future SES, etc.)
// consume the Notification and decide how to render it — color side-bars,
// emoji prefixes, channel routing, email subject lines, page severity.
//
// This package has no imports of any transport — parsers depend on it
// freely; transports depend on it; transports also depend on parsers; no
// cycles arise because parsers never import a transport.
package notify

// Severity classifies a notification's operational urgency. Parsers
// translate AWS-domain signals (CloudWatch state, GuardDuty score, build
// status, etc.) into one of these five buckets; transports decide how to
// surface each.
type Severity int

// Severity values, ordered from lowest to highest urgency. SeverityUnknown
// is the defensive zero value — a Notification whose Severity is never set
// renders as Unknown so it is impossible to silently emit an alert with no
// classification.
const (
	SeverityUnknown  Severity = iota // zero value; defensive default
	SeverityInfo                     // routine / FYI
	SeverityOK                       // explicit success state
	SeverityNotice                   // progress worth seeing, not actionable
	SeverityWarning                  // degraded / awaiting decision
	SeverityCritical                 // production-impacting / page-worthy
)

// String returns the canonical name for log fields and debug output.
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityOK:
		return "ok"
	case SeverityNotice:
		return "notice"
	case SeverityWarning:
		return "warning"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Field is one key/value row in a Notification's Fields list. Transports
// render fields in registration order; the slack renderer chunks them
// across multiple Block Kit sections to respect Slack's 10-field cap.
type Field struct {
	Key   string
	Value string
}

// Notification is the transport-neutral message a parser emits.
//
// Markup convention: Title, Subtitle, Summary, and Field values may
// embed a small mrkdwn-compatible subset — `*bold*`, “ `code` “, and
// the link form produced by Link(url, label). The Slack renderer passes
// these through verbatim; future plain-text transports strip them.
type Notification struct {
	Source    string   // parser identifier ("cloudwatch", "guardduty", …)
	Severity  Severity // operational urgency; drives transport visuals
	Title     string   // short headline; transports decorate (emoji, link)
	TitleURL  string   // optional click-through for the headline
	Subtitle  string   // small contextual line under the title
	Summary   string   // long-form body; mrkdwn-compatible subset
	Fields    []Field  // key/value rows
	ImageURL  string   // optional pre-signed chart URL
	Fallback  string   // plain-text rendering for low-fidelity transports
	Footnotes []string // small contextual lines (timestamps, etc.)
}
