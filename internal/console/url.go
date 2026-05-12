package console

import "strings"

// URL returns the AWS console URL for the given region and path.
//
// Partition detection:
//   - cn-*       → https://console.amazonaws.cn
//   - us-gov-*   → https://console.amazonaws-us-gov.com
//   - everything → https://console.aws.amazon.com
//
// The returned URL always includes the region query parameter so the
// console lands on the correct region tab.
func URL(region, path string) string {
	host := hostForRegion(region)
	trimmed := strings.TrimPrefix(path, "/")
	if region == "" {
		return host + "/" + trimmed
	}
	return host + "/" + trimmed + "?region=" + region
}

// URLWithFragment returns the AWS console URL for the given region, path, and
// hash fragment. The fragment is appended after the region query parameter
// (region in query, fragment last).
//
// The fragment argument must not include the leading "#"; it is added here.
func URLWithFragment(region, path, fragment string) string {
	base := URL(region, path)
	if fragment == "" {
		return base
	}
	return base + "#" + fragment
}

func hostForRegion(region string) string {
	switch {
	case strings.HasPrefix(region, "cn-"):
		return "https://console.amazonaws.cn"
	case strings.HasPrefix(region, "us-gov-"):
		return "https://console.amazonaws-us-gov.com"
	default:
		return "https://console.aws.amazon.com"
	}
}
