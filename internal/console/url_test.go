package console

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestURL_PartitionRouting(t *testing.T) {
	cases := []struct {
		name   string
		region string
		path   string
		want   string
	}{
		{
			name:   "commercial",
			region: "us-east-1",
			path:   "cloudwatch/home",
			want:   "https://console.aws.amazon.com/cloudwatch/home?region=us-east-1",
		},
		{
			name:   "china",
			region: "cn-north-1",
			path:   "cloudwatch/home",
			want:   "https://console.amazonaws.cn/cloudwatch/home?region=cn-north-1",
		},
		{
			name:   "govcloud",
			region: "us-gov-west-1",
			path:   "cloudwatch/home",
			want:   "https://console.amazonaws-us-gov.com/cloudwatch/home?region=us-gov-west-1",
		},
		{
			name:   "leading-slash-trimmed",
			region: "eu-west-1",
			path:   "/codepipeline/home",
			want:   "https://console.aws.amazon.com/codepipeline/home?region=eu-west-1",
		},
		{
			name:   "empty-region",
			region: "",
			path:   "billing/home",
			want:   "https://console.aws.amazon.com/billing/home",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := URL(tc.region, tc.path)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("URL mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestURLWithFragment(t *testing.T) {
	cases := []struct {
		name     string
		region   string
		path     string
		fragment string
		want     string
	}{
		{
			name:     "commercial-with-fragment",
			region:   "us-east-1",
			path:     "ec2/autoscaling/home",
			fragment: "AutoScalingGroups:id=example-asg",
			want:     "https://console.aws.amazon.com/ec2/autoscaling/home?region=us-east-1#AutoScalingGroups:id=example-asg",
		},
		{
			name:     "empty-fragment-falls-back-to-url",
			region:   "us-east-1",
			path:     "billing/home",
			fragment: "",
			want:     "https://console.aws.amazon.com/billing/home?region=us-east-1",
		},
		{
			name:     "empty-region-keeps-fragment",
			region:   "",
			path:     "billing/home",
			fragment: "summary",
			want:     "https://console.aws.amazon.com/billing/home#summary",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := URLWithFragment(tc.region, tc.path, tc.fragment)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("URLWithFragment mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
