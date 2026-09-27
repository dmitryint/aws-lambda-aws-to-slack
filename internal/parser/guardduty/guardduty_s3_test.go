package guardduty

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
)

func TestGuardDuty_S3BucketResource_RendersBucketNotRawJSON(t *testing.T) {
	cases := []struct {
		sample string
		bucket string
	}{
		{sample: "resource_s3_bucket.json", bucket: "example-bucket"},
		{sample: "s3_anomalous_behavior.json", bucket: "example-data-bucket"},
	}
	for _, tc := range cases {
		t.Run(tc.sample, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(samplesRoot, tc.sample)) //nolint:gosec // test fixture path
			if err != nil {
				t.Fatalf("read sample: %v", err)
			}
			ev, err := envelope.New(raw)
			if err != nil {
				t.Fatalf("envelope.New: %v", err)
			}
			msg, err := New().Parse(context.Background(), ev.Records()[0])
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			bucketShown := false
			for _, f := range msg.Fields {
				if strings.HasPrefix(f.Key, "Unknown Resource Type") {
					t.Errorf("S3Bucket resource rendered through the unknown-resource fallback: field %q", f.Key)
				}
				if strings.HasPrefix(strings.TrimSpace(f.Value), "{") {
					t.Errorf("field %q carries a raw JSON dump (%d bytes)", f.Key, len(f.Value))
				}
				if strings.Contains(f.Value, tc.bucket) {
					bucketShown = true
				}
			}
			if !bucketShown {
				t.Errorf("no field shows the affected bucket %q", tc.bucket)
			}
		})
	}
}
