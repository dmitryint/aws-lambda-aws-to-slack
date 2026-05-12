package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sync/errgroup"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/kms"
)

// Config holds the cold-start configuration the handler depends on.
//
// All values are read from environment variables. SLACK_HOOK_URL is the
// only required value; other fields are required by specific parsers and
// validated when those parsers run.
//
// SLACK_HOOK_URL and SLACK_CHANNEL are auto-decrypted at cold start when
// they look like KMS ciphertext. All other env vars are read as plaintext.
type Config struct {
	SlackHookURL      string
	SlackChannel      string
	ChartBucketName   string
	ChartBucketRegion string
	DedupTableName    string
	DedupTTLDays      int
	HideAWSLinks      string
	FunctionName      string
	Region            string
}

// envSlackHookURL and envSlackChannel are the env var names that may carry
// KMS-encrypted values. Kept as named constants so error messages and KMS
// decrypt calls reference the same string.
const (
	envSlackHookURL = "SLACK_HOOK_URL"
	envSlackChannel = "SLACK_CHANNEL"
)

// errMissingSlackHookURL is returned when SLACK_HOOK_URL is unset after
// decryption. Exposed as a sentinel for tests; production callers only
// inspect its message via fmt.
var errMissingSlackHookURL = errors.New(envSlackHookURL + " is required")

// Load reads configuration from the process environment, decrypting
// SLACK_HOOK_URL and SLACK_CHANNEL in parallel when they look like KMS
// ciphertext. Returns an error if either decrypt fails or if SLACK_HOOK_URL
// is empty after decryption.
//
// The two KMS calls fire concurrently via errgroup so cold-start latency
// is bounded by the slower of the two, not the sum.
func Load(ctx context.Context, d kms.Decrypter) (*Config, error) {
	rawHook := os.Getenv(envSlackHookURL)
	rawChannel := os.Getenv(envSlackChannel)

	var (
		hook    string
		channel string
	)
	group, gctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		v, err := kms.MaybeDecrypt(gctx, d, envSlackHookURL, rawHook)
		if err != nil {
			return err
		}
		hook = v
		return nil
	})
	group.Go(func() error {
		v, err := kms.MaybeDecrypt(gctx, d, envSlackChannel, rawChannel)
		if err != nil {
			return err
		}
		channel = v
		return nil
	})
	if err := group.Wait(); err != nil {
		return nil, fmt.Errorf("config: load: %w", err)
	}

	if hook == "" {
		return nil, errMissingSlackHookURL
	}

	cfg := &Config{
		SlackHookURL:      hook,
		SlackChannel:      channel,
		ChartBucketName:   os.Getenv("CHART_BUCKET_NAME"),
		ChartBucketRegion: os.Getenv("CHART_BUCKET_REGION"),
		DedupTableName:    os.Getenv("DEDUP_TABLE_NAME"),
		HideAWSLinks:      os.Getenv("HIDE_AWS_LINKS"),
		FunctionName:      os.Getenv("AWS_LAMBDA_FUNCTION_NAME"),
		Region:            os.Getenv("AWS_REGION"),
	}

	if raw := os.Getenv("DEDUP_TTL_DAYS"); raw != "" {
		ttl, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("DEDUP_TTL_DAYS must be an integer: %w", err)
		}
		cfg.DedupTTLDays = ttl
	}

	return cfg, nil
}
