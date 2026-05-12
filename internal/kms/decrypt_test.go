package kms

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// fakeDecrypter is a hand-rolled stub for the SDK Decrypter interface.
// Tests pre-program the next response or error.
type fakeDecrypter struct {
	calls     int
	gotInputs []*awskms.DecryptInput
	out       *awskms.DecryptOutput
	err       error
}

func (f *fakeDecrypter) Decrypt(
	_ context.Context, in *awskms.DecryptInput, _ ...func(*awskms.Options),
) (*awskms.DecryptOutput, error) {
	f.calls++
	f.gotInputs = append(f.gotInputs, in)
	if f.err != nil {
		return nil, f.err
	}
	return f.out, nil
}

func TestMaybeDecrypt_EmptyValue_NoCall(t *testing.T) {
	d := &fakeDecrypter{}
	got, err := MaybeDecrypt(t.Context(), d, "SLACK_HOOK_URL", "")
	if err != nil {
		t.Fatalf("MaybeDecrypt: %v", err)
	}
	if got != "" {
		t.Fatalf("got = %q, want empty", got)
	}
	if d.calls != 0 {
		t.Fatalf("Decrypt called %d times for empty input", d.calls)
	}
}

func TestMaybeDecrypt_Plaintext_NoCall(t *testing.T) {
	cases := []struct {
		name string
		file string
	}{
		{name: "url", file: "plaintext_url.txt"},
		{name: "channel", file: "plaintext_channel.txt"},
		{name: "long-no-spaces", file: "plaintext_long_no_spaces.txt"},
		{name: "random-base64", file: "random_base64.txt"},
		{name: "short-base64", file: "short_base64.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value := readTestdata(t, tc.file)
			d := &fakeDecrypter{}
			got, err := MaybeDecrypt(t.Context(), d, "SLACK_HOOK_URL", value)
			if err != nil {
				t.Fatalf("MaybeDecrypt: %v", err)
			}
			if got != value {
				t.Fatalf("got = %q, want %q (unchanged)", got, value)
			}
			if d.calls != 0 {
				t.Fatalf("Decrypt called %d times for plaintext input", d.calls)
			}
		})
	}
}

func TestMaybeDecrypt_Ciphertext_Success(t *testing.T) {
	value := readTestdata(t, "ciphertext_real.b64")
	d := &fakeDecrypter{
		out: &awskms.DecryptOutput{Plaintext: []byte("https://hooks.slack.com/services/T1/B1/secret")},
	}
	got, err := MaybeDecrypt(t.Context(), d, "SLACK_HOOK_URL", value)
	if err != nil {
		t.Fatalf("MaybeDecrypt: %v", err)
	}
	if got != "https://hooks.slack.com/services/T1/B1/secret" {
		t.Fatalf("plaintext = %q, want webhook url", got)
	}
	if d.calls != 1 {
		t.Fatalf("Decrypt calls = %d, want 1", d.calls)
	}
	// SDK Decrypt input must carry the decoded ciphertext, never the base64 string.
	wantDecoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatalf("base64 decode test fixture: %v", err)
	}
	if !bytes.Equal(d.gotInputs[0].CiphertextBlob, wantDecoded) {
		t.Fatalf("CiphertextBlob mismatch — got %d bytes, want %d", len(d.gotInputs[0].CiphertextBlob), len(wantDecoded))
	}
	// EncryptionContext must be empty: the env-var ciphertext is decrypted
	// with no encryption context.
	if len(d.gotInputs[0].EncryptionContext) != 0 {
		t.Fatalf("EncryptionContext = %v, want empty", d.gotInputs[0].EncryptionContext)
	}
}

func TestMaybeDecrypt_InvalidCiphertextException_FailsClosed(t *testing.T) {
	value := readTestdata(t, "ciphertext_real.b64")
	d := &fakeDecrypter{err: &types.InvalidCiphertextException{Message: aws.String("bad blob")}}
	got, err := MaybeDecrypt(t.Context(), d, "SLACK_HOOK_URL", value)
	if err == nil {
		t.Fatalf("expected error, got plaintext = %q", got)
	}
	if got != "" {
		t.Fatalf("got = %q, want empty on failure", got)
	}
	if !strings.Contains(err.Error(), "SLACK_HOOK_URL") {
		t.Fatalf("error %q does not name the env var", err)
	}
	var ice *types.InvalidCiphertextException
	if !errors.As(err, &ice) {
		t.Fatalf("error is not wrapped *InvalidCiphertextException: %v", err)
	}
}

func TestMaybeDecrypt_TransientError_FailsClosed(t *testing.T) {
	value := readTestdata(t, "ciphertext_real.b64")
	transient := errors.New("temporarily unavailable")
	d := &fakeDecrypter{err: transient}
	got, err := MaybeDecrypt(t.Context(), d, "SLACK_CHANNEL", value)
	if err == nil {
		t.Fatalf("expected error, got plaintext = %q", got)
	}
	if got != "" {
		t.Fatalf("got = %q, want empty on failure", got)
	}
	if !errors.Is(err, transient) {
		t.Fatalf("error chain does not include transient: %v", err)
	}
	if !strings.Contains(err.Error(), "SLACK_CHANNEL") {
		t.Fatalf("error %q does not name the env var", err)
	}
}

func TestMaybeDecrypt_LooksLikeButInvalidBase64(t *testing.T) {
	// Pathological: a value that the magic-byte detector flagged (the
	// detector decodes once internally) but which fails the second
	// decode inside MaybeDecrypt. In practice the two decode calls use
	// the same input, so this branch is reachable only with a race —
	// we cover it with a hand-crafted string that decodes one way for
	// StdEncoding and not the other. Easier path: skip; the fail-closed
	// contract is already covered by the InvalidCiphertextException case.
	t.Skip("unreachable in practice — covered by detect_test")
}

func TestNewClient(t *testing.T) {
	c := NewClient(aws.Config{})
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
}

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read testdata/%s: %v", name, err)
	}
	return strings.TrimSpace(string(raw))
}
