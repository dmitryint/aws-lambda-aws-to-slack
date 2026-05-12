package slack

// Color constants used by parsers to set the legacy attachment side-bar.
// Critical/Warning/OK match Slack's three reserved keywords; Accent and
// Neutral are hex values used for informational alerts.
const (
	ColorCritical = "danger"
	ColorWarning  = "warning"
	ColorOK       = "good"
	ColorAccent   = "#439FE0"
	ColorNeutral  = "#dddddd"
)
