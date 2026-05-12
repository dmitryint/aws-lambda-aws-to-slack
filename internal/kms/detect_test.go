package kms

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLooksLikeKMSCiphertext_Testdata(t *testing.T) {
	cases := []struct {
		name string
		file string
		want bool
	}{
		{name: "plaintext-url", file: "plaintext_url.txt", want: false},
		{name: "plaintext-channel", file: "plaintext_channel.txt", want: false},
		{name: "plaintext-long-no-spaces", file: "plaintext_long_no_spaces.txt", want: false},
		{name: "ciphertext-real", file: "ciphertext_real.b64", want: true},
		{name: "random-base64", file: "random_base64.txt", want: false},
		{name: "short-base64", file: "short_base64.txt", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", tc.file))
			if err != nil {
				t.Fatalf("read testdata/%s: %v", tc.file, err)
			}
			value := strings.TrimSpace(string(raw))
			if got := LooksLikeKMSCiphertext(value); got != tc.want {
				t.Fatalf("LooksLikeKMSCiphertext(%s) = %v, want %v", tc.file, got, tc.want)
			}
		})
	}
}

func TestLooksLikeKMSCiphertext(t *testing.T) {
	// A "ciphertext-shaped" blob: 0x01 0x02 header + enough padding to clear
	// the kmsMinCiphertextLen check. Real KMS blobs are larger, but the
	// detector only inspects the first two bytes and the overall length.
	ciphertext := make([]byte, kmsMinCiphertextLen+50)
	ciphertext[0] = 0x01
	ciphertext[1] = 0x02
	ciphertextB64 := base64.StdEncoding.EncodeToString(ciphertext)

	// Random bytes the same length but with a different prefix — must not match.
	random := make([]byte, kmsMinCiphertextLen+50)
	for i := range random {
		random[i] = byte(i % 251)
	}
	randomB64 := base64.StdEncoding.EncodeToString(random)

	// Short blob with the magic bytes but below the length threshold.
	shortB64 := base64.StdEncoding.EncodeToString([]byte{0x01, 0x02, 'h', 'i'})

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{name: "empty", in: "", want: false},
		{name: "plaintext-url", in: "https://example.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX", want: false},
		{name: "plaintext-channel", in: "#alerts-prod", want: false},
		{name: "invalid-base64", in: "not base64 !!!", want: false},
		{name: "random-base64", in: randomB64, want: false},
		{name: "short-magic", in: shortB64, want: false},
		{name: "valid-magic-and-length", in: ciphertextB64, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LooksLikeKMSCiphertext(tc.in); got != tc.want {
				t.Fatalf("LooksLikeKMSCiphertext(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
