package envelope

import (
	"fmt"
	"strings"
)

// arnPartCount is the maximum number of colon-separated segments the parser
// keeps. strings.SplitN with n=7 packs anything past the 6th colon into
// the final element, preserving multi-segment resource suffixes.
const arnPartCount = 7

// minARNPartCount is the lower bound for a parseable ARN: partition,
// service, region, account, resource must all be present (the suffix is
// optional).
const minARNPartCount = 6

// ARN is the decomposed view of an AWS resource ARN.
//
// The 7th element absorbs any extra colons so SNS subscription IDs and
// other multi-segment resource suffixes stay intact.
type ARN struct {
	Partition string
	Service   string
	Region    string
	AccountID string
	Resource  string
	Suffix    string
}

// ParseARN decomposes an ARN into its parts.
//
// Returns the zero value when the input does not look like an ARN
// (missing prefix or fewer than 6 segments). SplitN(s, ":", 7) keeps any
// suffix after the 6th colon intact.
func ParseARN(s string) ARN {
	arn, _ := tryParseARN(s)
	return arn
}

// tryParseARN is the same as ParseARN but surfaces a parse error for use
// inside table tests. Kept unexported because callers in the runtime use
// ParseARN and never inspect the error.
func tryParseARN(s string) (ARN, error) {
	if !strings.HasPrefix(s, "arn:") {
		return ARN{}, fmt.Errorf("arn must start with 'arn:': %q", s)
	}
	parts := strings.SplitN(s, ":", arnPartCount)
	if len(parts) < minARNPartCount {
		return ARN{}, fmt.Errorf("arn has fewer than %d segments: %q", minARNPartCount, s)
	}
	out := ARN{
		Partition: parts[1],
		Service:   parts[2],
		Region:    parts[3],
		AccountID: parts[4],
		Resource:  parts[5],
	}
	if len(parts) == arnPartCount {
		out.Suffix = parts[6]
	}
	return out, nil
}
