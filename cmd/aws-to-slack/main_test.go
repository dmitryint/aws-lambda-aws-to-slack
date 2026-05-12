package main

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/handler"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/kms"
)

// fakeDecrypter is a hand-rolled Decrypter for main_test. The plaintext
// is keyed by the decoded ciphertext bytes so the assertion matches the
// real production call shape (CiphertextBlob, not the base64 string).
type fakeDecrypter struct {
	plaintext map[string]string
	err       error
}

func (f *fakeDecrypter) Decrypt(
	_ context.Context, in *awskms.DecryptInput, _ ...func(*awskms.Options),
) (*awskms.DecryptOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	if pt, ok := f.plaintext[string(in.CiphertextBlob)]; ok {
		return &awskms.DecryptOutput{Plaintext: []byte(pt)}, nil
	}
	return &awskms.DecryptOutput{Plaintext: in.CiphertextBlob}, nil
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "kms", "testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return strings.TrimSpace(string(raw))
}

func TestRun_HappyPath(t *testing.T) {
	t.Setenv("SLACK_HOOK_URL", "https://hooks.slack.com/services/T1/B1/abcd")
	t.Setenv("SLACK_CHANNEL", "#alerts")

	var started bool
	deps := runDeps{
		awsLoader:  func(ctx context.Context) (aws.Config, error) { return aws.Config{}, nil },
		kmsFactory: func(aws.Config) kms.Decrypter { return &fakeDecrypter{} },
		starter:    func(*handler.Handler) { started = true },
	}
	if err := run(t.Context(), deps); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !started {
		t.Fatal("lambda.Start equivalent not invoked")
	}
}

func TestRun_AWSConfigLoadFails(t *testing.T) {
	t.Setenv("SLACK_HOOK_URL", "https://hooks.slack.com/services/T1/B1/abcd")

	awsErr := errors.New("ec2 metadata: no role")
	deps := runDeps{
		awsLoader:  func(ctx context.Context) (aws.Config, error) { return aws.Config{}, awsErr },
		kmsFactory: func(aws.Config) kms.Decrypter { return &fakeDecrypter{} },
		starter:    func(*handler.Handler) {},
	}
	err := run(t.Context(), deps)
	if err == nil {
		t.Fatal("expected aws config error")
	}
	if !errors.Is(err, awsErr) {
		t.Fatalf("aws error not wrapped: %v", err)
	}
}

func TestRun_KMSDecryptFails_FailClosed(t *testing.T) {
	ciphertext := readFixture(t, "ciphertext_real.b64")
	t.Setenv("SLACK_HOOK_URL", ciphertext)
	t.Setenv("SLACK_CHANNEL", "")

	deps := runDeps{
		awsLoader: func(ctx context.Context) (aws.Config, error) { return aws.Config{}, nil },
		kmsFactory: func(aws.Config) kms.Decrypter {
			return &fakeDecrypter{err: &types.InvalidCiphertextException{Message: aws.String("bad")}}
		},
		starter: func(*handler.Handler) {
			t.Fatal("starter must not run when config.Load fails")
		},
	}
	err := run(t.Context(), deps)
	if err == nil {
		t.Fatal("expected fail-closed error")
	}
	if !strings.Contains(err.Error(), "SLACK_HOOK_URL") {
		t.Fatalf("error %q does not name the env var", err)
	}
	var ice *types.InvalidCiphertextException
	if !errors.As(err, &ice) {
		t.Fatalf("error chain missing *InvalidCiphertextException: %v", err)
	}
}

func TestRun_MissingHookURL(t *testing.T) {
	t.Setenv("SLACK_HOOK_URL", "")
	t.Setenv("SLACK_CHANNEL", "")

	deps := runDeps{
		awsLoader:  func(ctx context.Context) (aws.Config, error) { return aws.Config{}, nil },
		kmsFactory: func(aws.Config) kms.Decrypter { return &fakeDecrypter{} },
		starter:    func(*handler.Handler) { t.Fatal("starter must not run without webhook url") },
	}
	if err := run(t.Context(), deps); err == nil {
		t.Fatal("expected error for missing SLACK_HOOK_URL")
	}
}

func TestRun_DecryptsCiphertextThenStarts(t *testing.T) {
	ciphertext := readFixture(t, "ciphertext_real.b64")
	t.Setenv("SLACK_HOOK_URL", ciphertext)
	t.Setenv("SLACK_CHANNEL", "")

	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	plaintext := "https://hooks.slack.com/services/T1/B1/decrypted"

	var started bool
	deps := runDeps{
		awsLoader: func(ctx context.Context) (aws.Config, error) { return aws.Config{}, nil },
		kmsFactory: func(aws.Config) kms.Decrypter {
			return &fakeDecrypter{plaintext: map[string]string{string(decoded): plaintext}}
		},
		starter: func(h *handler.Handler) {
			if h == nil {
				t.Fatal("starter received nil handler")
			}
			started = true
		},
	}
	if err := run(t.Context(), deps); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !started {
		t.Fatal("starter not invoked after decrypt")
	}
}

func TestExecute_HappyPath_ReturnsZero(t *testing.T) {
	t.Setenv("SLACK_HOOK_URL", "https://hooks.slack.com/services/T1/B1/abcd")
	t.Setenv("SLACK_CHANNEL", "")

	deps := runDeps{
		awsLoader:  func(ctx context.Context) (aws.Config, error) { return aws.Config{}, nil },
		kmsFactory: func(aws.Config) kms.Decrypter { return &fakeDecrypter{} },
		starter:    func(*handler.Handler) {},
	}
	if code := execute(t.Context(), deps); code != 0 {
		t.Fatalf("execute exit code = %d, want 0", code)
	}
}

func TestExecute_Failure_ReturnsNonZero(t *testing.T) {
	t.Setenv("SLACK_HOOK_URL", "")
	t.Setenv("SLACK_CHANNEL", "")

	deps := runDeps{
		awsLoader:  func(ctx context.Context) (aws.Config, error) { return aws.Config{}, nil },
		kmsFactory: func(aws.Config) kms.Decrypter { return &fakeDecrypter{} },
		starter:    func(*handler.Handler) { t.Fatal("starter must not run") },
	}
	if code := execute(t.Context(), deps); code == 0 {
		t.Fatal("expected non-zero exit code for missing hook url")
	}
}

func TestProductionDeps_HasAllSeams(t *testing.T) {
	// Exercise the production wiring once so coverage reflects the real
	// factory functions. We can't actually invoke the starter — it
	// would block on the Lambda runtime API — but awsLoader and
	// kmsFactory are safe to call here.
	d := productionDeps()
	if d.awsLoader == nil || d.kmsFactory == nil || d.starter == nil {
		t.Fatalf("productionDeps has nil seams: %+v", d)
	}
	// kmsFactory wraps the empty SDK config into a real *kms.Client.
	if got := d.kmsFactory(aws.Config{}); got == nil {
		t.Fatal("kmsFactory returned nil client")
	}
	// awsLoader builds a default SDK config. It is safe to invoke even
	// without AWS credentials — it returns a usable but unauthenticated
	// config rather than erroring.
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if _, err := d.awsLoader(ctx); err != nil {
		t.Fatalf("awsLoader returned %v", err)
	}
}
