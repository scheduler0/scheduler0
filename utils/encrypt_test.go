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

func TestGenerateApiAndSecretKey(t *testing.T) {
	apiKey, secretKey := GenerateApiAndSecretKey(testKeyString)

	// Test generated API key
	decryptedApiKey := Decrypt(apiKey, testKeyString)
	if decryptedApiKey == apiKey {
		t.Fatalf("API key generation failed: decrypted API key is the same as the encrypted API key")
	}

	// Test generated secret key
	decryptedSecretKey := Decrypt(secretKey, testKeyString)
	if decryptedSecretKey == secretKey {
		t.Fatalf("Secret key generation failed: decrypted secret key is the same as the encrypted secret key")
	}
}
