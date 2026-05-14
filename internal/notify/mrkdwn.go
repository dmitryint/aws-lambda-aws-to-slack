package notify

import "strings"

// Link builds a Slack-compatible mrkdwn link `<url|label>`. The Slack
// renderer passes the string through verbatim; plain-text transports
// extract the label.
//
// Returns the bare label when url is empty or label is empty. Pipe
// characters in either input are stripped — Slack treats them as the
// url/label separator and a stray one would corrupt the link.
func Link(url, label string) string {
	if label == "" {
		label = url
	}
	if url == "" {
		return strings.ReplaceAll(label, "|", "")
	}
	url = strings.ReplaceAll(url, "|", "")
	label = strings.ReplaceAll(label, "|", "")
	return "<" + url + "|" + label + ">"
}
