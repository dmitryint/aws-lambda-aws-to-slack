package config

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// stubDecrypter is a hand-rolled Decrypter used by all config tests.
// onCall lets a test fail when no decrypt was expected (plaintext path).
type stubDecrypter struct {
	calls     atomic.Int32
	plaintext map[string]string
	fail      map[string]error
}

func (s *stubDecrypter) Decrypt(
	ctx context.Context, in *awskms.DecryptInput, _ ...func(*awskms.Options),
) (*awskms.DecryptOutput, error) {
	s.calls.Add(1)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	key := string(in.CiphertextBlob)
	if err, ok := s.fail[key]; ok {
		return nil, err
	}
	pt, ok := s.plaintext[key]
	if !ok {
		// Fallback: return the input blob (unrealistic but never used by
		// tests that don't pre-program plaintext for the input).
		return &awskms.DecryptOutput{Plaintext: in.CiphertextBlob}, nil
	}
	return &awskms.DecryptOutput{Plaintext: []byte(pt)}, nil
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "kms", "testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return strings.TrimSpace(string(raw))
}

func TestLoad_PlaintextHookURL_NoDecrypt(t *testing.T) {
	t.Setenv(envSlackHookURL, "https://hooks.slack.com/services/T1/B1/abcd")
	t.Setenv(envSlackChannel, "")

	d := &stubDecrypter{}
	cfg, err := Load(t.Context(), d)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SlackHookURL != "https://hooks.slack.com/services/T1/B1/abcd" {
		t.Fatalf("SlackHookURL = %q", cfg.SlackHookURL)
	}
	if cfg.SlackChannel != "" {
		t.Fatalf("SlackChannel = %q, want empty", cfg.SlackChannel)
	}
	if got := d.calls.Load(); got != 0 {
		t.Fatalf("Decrypt called %d times for plaintext input", got)
	}
}

func TestLoad_CiphertextHookURL_Decrypts(t *testing.T) {
	ciphertext := readFixture(t, "ciphertext_real.b64")
	t.Setenv(envSlackHookURL, ciphertext)
	t.Setenv(envSlackChannel, "")

	// Pre-program the stub keyed by the decoded ciphertext bytes.
	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	d := &stubDecrypter{
		plaintext: map[string]string{
			string(decoded): "https://hooks.slack.com/services/T1/B1/secret",
		},
	}

	cfg, err := Load(t.Context(), d)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SlackHookURL != "https://hooks.slack.com/services/T1/B1/secret" {
		t.Fatalf("SlackHookURL = %q, want decrypted plaintext", cfg.SlackHookURL)
	}
	if got := d.calls.Load(); got != 1 {
		t.Fatalf("Decrypt calls = %d, want 1", got)
	}
}

func TestLoad_MissingHookURL_Errors(t *testing.T) {
	t.Setenv(envSlackHookURL, "")
	t.Setenv(envSlackChannel, "")

	d := &stubDecrypter{}
	cfg, err := Load(t.Context(), d)
	if err == nil {
		t.Fatalf("expected error, got cfg = %+v", cfg)
	}
	if !errors.Is(err, errMissingSlackHookURL) {
		t.Fatalf("error = %v, want %v", err, errMissingSlackHookURL)
	}
}

func TestLoad_KMSFailsOnHookURL_FailClosed(t *testing.T) {
	ciphertext := readFixture(t, "ciphertext_real.b64")
	t.Setenv(envSlackHookURL, ciphertext)
	t.Setenv(envSlackChannel, "")

	d := &stubDecrypter{
		fail: map[string]error{
			ciphertextKey(t, ciphertext): &types.InvalidCiphertextException{Message: aws.String("bad")},
		},
	}
	cfg, err := Load(t.Context(), d)
	if err == nil {
		t.Fatalf("expected error, got cfg = %+v", cfg)
	}
	if !strings.Contains(err.Error(), envSlackHookURL) {
		t.Fatalf("error %q does not name %s", err, envSlackHookURL)
	}
	var ice *types.InvalidCiphertextException
	if !errors.As(err, &ice) {
		t.Fatalf("error chain missing InvalidCiphertextException: %v", err)
	}
}

func TestLoad_KMSFailsOnChannel_FailClosed(t *testing.T) {
	ciphertext := readFixture(t, "ciphertext_real.b64")
	t.Setenv(envSlackHookURL, "https://hooks.slack.com/services/T1/B1/abcd")
	t.Setenv(envSlackChannel, ciphertext)

	d := &stubDecrypter{
		fail: map[string]error{
			ciphertextKey(t, ciphertext): errors.New("transient kms outage"),
		},
	}
	cfg, err := Load(t.Context(), d)
	if err == nil {
		t.Fatalf("expected error, got cfg = %+v", cfg)
	}
	if !strings.Contains(err.Error(), envSlackChannel) {
		t.Fatalf("error %q does not name %s", err, envSlackChannel)
	}
}

func TestLoad_CtxCancellation_Propagates(t *testing.T) {
	ciphertext := readFixture(t, "ciphertext_real.b64")
	t.Setenv(envSlackHookURL, ciphertext)
	t.Setenv(envSlackChannel, "")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	d := &stubDecrypter{}
	if _, err := Load(ctx, d); err == nil {
		t.Fatalf("expected context error")
	}
}

func TestLoad_PlumbsPlaintextFields(t *testing.T) {
	t.Setenv(envSlackHookURL, "https://hooks.slack.com/services/T1/B1/abcd")
	t.Setenv(envSlackChannel, "#alerts-prod")
	t.Setenv("CHART_BUCKET_NAME", "esai-charts")
	t.Setenv("CHART_BUCKET_REGION", "us-east-2")
	t.Setenv("DEDUP_TABLE_NAME", "inspector2-dedup")
	t.Setenv("DEDUP_TTL_DAYS", "7")
	t.Setenv("HIDE_AWS_LINKS", "true")
	t.Setenv("AWS_LAMBDA_FUNCTION_NAME", "aws-to-slack")
	t.Setenv("AWS_REGION", "us-east-2")

	cfg, err := Load(t.Context(), &stubDecrypter{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ChartBucketName != "esai-charts" || cfg.ChartBucketRegion != "us-east-2" {
		t.Fatalf("chart cfg wrong: %+v", cfg)
	}
	if cfg.DedupTableName != "inspector2-dedup" || cfg.DedupTTLDays != 7 {
		t.Fatalf("dedup cfg wrong: %+v", cfg)
	}
	if cfg.HideAWSLinks != "true" || cfg.FunctionName != "aws-to-slack" || cfg.Region != "us-east-2" {
		t.Fatalf("runtime cfg wrong: %+v", cfg)
	}
}

func TestLoad_DedupTTLDays_NotInteger(t *testing.T) {
	t.Setenv(envSlackHookURL, "https://hooks.slack.com/services/T1/B1/abcd")
	t.Setenv(envSlackChannel, "")
	t.Setenv("DEDUP_TTL_DAYS", "not-a-number")

	_, err := Load(t.Context(), &stubDecrypter{})
	if err == nil {
		t.Fatalf("expected error for invalid DEDUP_TTL_DAYS")
	}
	if !strings.Contains(err.Error(), "DEDUP_TTL_DAYS") {
		t.Fatalf("error %q does not name DEDUP_TTL_DAYS", err)
	}
}

// ciphertextKey returns the same byte string the stubDecrypter keys on —
// the decoded ciphertext blob — so test setup mirrors what MaybeDecrypt
// passes to Decrypt.
func ciphertextKey(t *testing.T, b64 string) string {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}
	return string(decoded)
}
