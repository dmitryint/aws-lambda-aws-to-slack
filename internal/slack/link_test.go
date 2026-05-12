package slack

import "testing"

func TestLink_HideAWSLinksPredicate(t *testing.T) {
	cases := []struct {
		name      string
		envVal    string
		url       string
		text      string
		wantValue string
	}{
		{name: "empty-env-shows-link", envVal: "", url: "https://x", text: "X", wantValue: "<https://x|X>"},
		{name: "true-lower-hides", envVal: "true", url: "https://x", text: "X", wantValue: "X"},
		{name: "true-upper-hides", envVal: "TRUE", url: "https://x", text: "X", wantValue: "X"},
		{name: "true-mixed-hides", envVal: "True", url: "https://x", text: "X", wantValue: "X"},
		{name: "one-hides", envVal: "1", url: "https://x", text: "X", wantValue: "X"},
		{name: "zero-shows", envVal: "0", url: "https://x", text: "X", wantValue: "<https://x|X>"},
		{name: "false-shows", envVal: "false", url: "https://x", text: "X", wantValue: "<https://x|X>"},
		{name: "yes-shows", envVal: "yes", url: "https://x", text: "X", wantValue: "<https://x|X>"},
		{name: "empty-url-returns-text-even-when-shown", envVal: "", url: "", text: "X", wantValue: "X"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HIDE_AWS_LINKS", tc.envVal)
			if got := Link(tc.url, tc.text); got != tc.wantValue {
				t.Fatalf("Link(%q, %q) with HIDE_AWS_LINKS=%q = %q, want %q",
					tc.url, tc.text, tc.envVal, got, tc.wantValue)
			}
		})
	}
}
