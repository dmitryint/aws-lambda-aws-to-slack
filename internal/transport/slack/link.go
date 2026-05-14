package slack

import (
	"os"
	"regexp"
)

// hideAWSLinks matches the HIDE_AWS_LINKS truthy predicate:
// case-insensitive "true" or the literal "1". Anything else falls through
// to "show links".
var hideAWSLinks = regexp.MustCompile(`(?i)^(true|1)$`)

// hideLinksEnabled reports whether the HIDE_AWS_LINKS environment variable
// is set to a truthy value. When enabled the Slack renderer strips the URL
// from every mrkdwn <url|label> segment it emits — useful when console
// links would leak account context into less-trusted channels.
func hideLinksEnabled() bool {
	return hideAWSLinks.MatchString(os.Getenv("HIDE_AWS_LINKS"))
}
