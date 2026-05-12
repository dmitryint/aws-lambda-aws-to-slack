package slack

import (
	"os"
	"regexp"
)

// hideAWSLinks matches the HIDE_AWS_LINKS truthy predicate:
// case-insensitive "true" or the literal "1". Anything else falls through
// to "show links".
var hideAWSLinks = regexp.MustCompile(`(?i)^(true|1)$`)

// Link formats a Slack <url|text> mrkdwn link. When the HIDE_AWS_LINKS env
// var is set to a truthy value, the URL is suppressed and only the text is
// returned — useful when AWS console links would leak account context into
// less-trusted channels.
func Link(url, text string) string {
	if hideAWSLinks.MatchString(os.Getenv("HIDE_AWS_LINKS")) {
		return text
	}
	if url == "" {
		return text
	}
	return "<" + url + "|" + text + ">"
}
