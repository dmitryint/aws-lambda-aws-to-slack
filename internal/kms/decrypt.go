package kms

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
)

// Decrypter is the seam between the config loader and the real KMS SDK
// client. Tests substitute a fake; production wires in the SDK v2
// `*kms.Client`, which satisfies this interface by signature.
//
// The shape mirrors the SDK so production code passes the SDK client
// directly with no adapter — see cmd/aws-to-slack/main.go.
type Decrypter interface {
	Decrypt(ctx context.Context, in *awskms.DecryptInput, optFns ...func(*awskms.Options)) (*awskms.DecryptOutput, error)
}

// NewClient is the single production constructor for the SDK KMS client.
// Centralizing the factory here means cmd/aws-to-slack/main.go does not
// need to import the SDK kms package directly — every cold-start path
// goes through this seam.
func NewClient(cfg aws.Config) *awskms.Client {
	return awskms.NewFromConfig(cfg)
}

// MaybeDecrypt returns value unchanged when LooksLikeKMSCiphertext is
// false, base64-decodes and calls Decrypt with no EncryptionContext when
// it is true, and wraps any decrypt error with varName for the operator.
//
// Fail-closed: on any decrypt error the original ciphertext is never
// returned to the caller — the error is the only observable. The Lambda
// init then exits non-zero, the runtime ticks the Errors metric, and the
// configured CloudWatch alarm fires.
func MaybeDecrypt(ctx context.Context, d Decrypter, varName, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !LooksLikeKMSCiphertext(value) {
		return value, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("kms: decode %s: %w", varName, err)
	}
	out, err := d.Decrypt(ctx, &awskms.DecryptInput{CiphertextBlob: decoded})
	if err != nil {
		return "", fmt.Errorf("kms: decrypt %s: %w", varName, err)
	}
	return string(out.Plaintext), nil
}
