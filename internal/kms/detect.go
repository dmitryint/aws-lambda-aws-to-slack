package kms

import "encoding/base64"

// kmsMinCiphertextLen is the lower bound for a KMS v1 ciphertext blob.
// The KMS-owned key ARN in the header alone takes 60-80 bytes; we round
// down to 100 to leave a safety margin while still rejecting random
// short base64 strings.
const kmsMinCiphertextLen = 100

// LooksLikeKMSCiphertext returns true when s is a base64 string whose
// decoded form matches the KMS v1 ciphertext magic header (0x01 0x02).
//
// The two-byte header has held since at least 2017. No reasonable plaintext
// (Slack URLs, channel names, opaque tokens) is both valid base64 and starts
// with 0x01 0x02 after decoding — so this is a deterministic predicate.
func LooksLikeKMSCiphertext(s string) bool {
	if s == "" {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return false
	}
	return len(decoded) >= kmsMinCiphertextLen && decoded[0] == 0x01 && decoded[1] == 0x02
}
