package envelope

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseARN_Table(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    ARN
		wantErr bool
	}{
		{
			name: "sns-topic-with-subscription-suffix",
			in:   "arn:aws:sns:us-east-1:123456789012:my-topic:abcd-1234",
			want: ARN{
				Partition: "aws",
				Service:   "sns",
				Region:    "us-east-1",
				AccountID: "123456789012",
				Resource:  "my-topic",
				Suffix:    "abcd-1234",
			},
		},
		{
			name: "sns-topic-no-suffix",
			in:   "arn:aws:sns:us-east-1:123456789012:my-topic",
			want: ARN{
				Partition: "aws",
				Service:   "sns",
				Region:    "us-east-1",
				AccountID: "123456789012",
				Resource:  "my-topic",
			},
		},
		{
			name: "iam-resource-with-colons-after-resource-keep-suffix",
			in:   "arn:aws:iam::123456789012:role:assumed-role:foo:bar",
			want: ARN{
				Partition: "aws",
				Service:   "iam",
				Region:    "",
				AccountID: "123456789012",
				Resource:  "role",
				Suffix:    "assumed-role:foo:bar",
			},
		},
		{
			name: "china-partition",
			in:   "arn:aws-cn:sns:cn-north-1:123456789012:topic",
			want: ARN{
				Partition: "aws-cn",
				Service:   "sns",
				Region:    "cn-north-1",
				AccountID: "123456789012",
				Resource:  "topic",
			},
		},
		{
			name: "govcloud-partition",
			in:   "arn:aws-us-gov:sns:us-gov-west-1:123456789012:topic",
			want: ARN{
				Partition: "aws-us-gov",
				Service:   "sns",
				Region:    "us-gov-west-1",
				AccountID: "123456789012",
				Resource:  "topic",
			},
		},
		{
			name:    "malformed-empty",
			in:      "",
			wantErr: true,
		},
		{
			name:    "malformed-prefix",
			in:      "not-an-arn",
			wantErr: true,
		},
		{
			name:    "malformed-too-few-segments",
			in:      "arn:aws:sns:us-east-1:123456789012",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tryParseARN(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("ARN mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseARN_PublicAPIReturnsZeroOnError(t *testing.T) {
	if got := ParseARN(""); got != (ARN{}) {
		t.Fatalf("ParseARN(\"\") = %+v, want zero", got)
	}
	if got := ParseARN("nonsense"); got != (ARN{}) {
		t.Fatalf("ParseARN(nonsense) = %+v, want zero", got)
	}
}
