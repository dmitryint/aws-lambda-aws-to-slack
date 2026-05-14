package slack

import "testing"

func TestHideLinksEnabled(t *testing.T) {
	cases := []struct {
		name   string
		envVal string
		want   bool
	}{
		{name: "empty-env-shows-link", envVal: "", want: false},
		{name: "true-lower-hides", envVal: "true", want: true},
		{name: "true-upper-hides", envVal: "TRUE", want: true},
		{name: "true-mixed-hides", envVal: "True", want: true},
		{name: "one-hides", envVal: "1", want: true},
		{name: "zero-shows", envVal: "0", want: false},
		{name: "false-shows", envVal: "false", want: false},
		{name: "yes-shows", envVal: "yes", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HIDE_AWS_LINKS", tc.envVal)
			if got := hideLinksEnabled(); got != tc.want {
				t.Fatalf("hideLinksEnabled() with HIDE_AWS_LINKS=%q = %v, want %v",
					tc.envVal, got, tc.want)
			}
		})
	}
}
