package utils

import (
	"testing"
)

const testKeyString = "6368616e676520746869732070617373776f726420746f206120736563726574"

func TestEncryptDecrypt(t *testing.T) {
	originalText := "This is a secret message"

	// Test encryption
	encryptedText := Encrypt(originalText, testKeyString)
	if encryptedText == originalText {
		t.Fatalf("Encryption failed: encrypted text is the same as original text")
	}

	// Test decryption
	decryptedText := Decrypt(encryptedText, testKeyString)
	if decryptedText != originalText {
		t.Fatalf("Decryption failed: expected '%s', got '%s'", originalText, decryptedText)
	}
}

func TestGenerateApiKey(t *testing.T) {
	apiKey := GenerateApiKey(testKeyString)
	if apiKey == "" {
		t.Fatal("GenerateApiKey returned an empty api key")
	}
	// The api key is an encrypted opaque identifier, so it must not equal its own plaintext.
	if decrypted := Decrypt(apiKey, testKeyString); decrypted == apiKey {
		t.Fatalf("API key generation failed: decrypted API key is the same as the encrypted API key")
	}
	// Two api keys must be distinct.
	if GenerateApiKey(testKeyString) == apiKey {
		t.Fatal("GenerateApiKey produced identical keys on successive calls")
	}
}

func TestGenerateApiSecret(t *testing.T) {
	plaintext, ciphertext := GenerateApiSecret(testKeyString)
	if plaintext == "" || ciphertext == "" {
		t.Fatal("GenerateApiSecret returned an empty plaintext or ciphertext")
	}
	if plaintext == ciphertext {
		t.Fatal("GenerateApiSecret returned plaintext equal to ciphertext")
	}
	// The stored ciphertext must decrypt back to the plaintext handed to the client — this
	// is what the auth path relies on (decrypt stored, compare to presented).
	if got := Decrypt(ciphertext, testKeyString); got != plaintext {
		t.Fatalf("GenerateApiSecret round-trip failed: expected %q, got %q", plaintext, got)
	}
}

// A second, distinct valid AES-256 hex key for rotation tests.
const testKeyString2 = "0000000000000000000000000000000000000000000000000000000000000001"

func TestIsValidAESHexKey(t *testing.T) {
	valid := []string{
		testKeyString,                                      // 32 bytes
		"00112233445566778899aabbccddeeff",                 // 16 bytes
		"00112233445566778899aabbccddeeff0011223344556677", // 24 bytes
	}
	for _, k := range valid {
		if !IsValidAESHexKey(k) {
			t.Fatalf("expected key %q to be valid", k)
		}
	}

	invalid := []string{
		"",               // empty
		"not-hex",        // not hex
		"abcd",           // too short (2 bytes)
		"00112233445566", // 7 bytes
	}
	for _, k := range invalid {
		if IsValidAESHexKey(k) {
			t.Fatalf("expected key %q to be invalid", k)
		}
	}
}

func TestReEncrypt(t *testing.T) {
	plaintext := "AKIAIOSFODNN7EXAMPLE"
	oldCipher := Encrypt(plaintext, testKeyString)

	// Happy path: value encrypted with the old key is re-encrypted under the new key.
	newCipher, ok := ReEncrypt(oldCipher, testKeyString, testKeyString2)
	if !ok {
		t.Fatalf("ReEncrypt returned ok=false for a value encrypted with the old key")
	}
	if newCipher == oldCipher {
		t.Fatalf("ReEncrypt produced identical ciphertext; expected a re-encryption")
	}
	// The new ciphertext must decrypt to the original plaintext under the new key...
	if got := Decrypt(newCipher, testKeyString2); got != plaintext {
		t.Fatalf("re-encrypted value did not round-trip: expected %q, got %q", plaintext, got)
	}
	// ...and must NOT be decryptable under the old key.
	func() {
		defer func() { _ = recover() }()
		if got := Decrypt(newCipher, testKeyString); got == plaintext {
			t.Fatalf("re-encrypted value should not decrypt to plaintext under the old key")
		}
	}()

	// Empty input is a no-op signalled by ok=false.
	if _, ok := ReEncrypt("", testKeyString, testKeyString2); ok {
		t.Fatalf("ReEncrypt on empty input should return ok=false")
	}

	// A value that cannot be decrypted with the old key (e.g. already rotated, or legacy
	// plaintext) is left alone: ok=false so callers don't corrupt it.
	if _, ok := ReEncrypt("plainlegacyvalue", testKeyString, testKeyString2); ok {
		t.Fatalf("ReEncrypt on an undecryptable value should return ok=false")
	}
}
