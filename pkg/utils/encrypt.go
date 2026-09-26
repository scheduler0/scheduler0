package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// Encrypt string
func Encrypt(stringToEncrypt string, keyString string) (encryptedString string) {
	//Since the key is in string, we need to convert decode it to bytes
	key, _ := hex.DecodeString(keyString)
	plaintext := []byte(stringToEncrypt)

	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}

	//Create a new GCM - https://en.wikipedia.org/wiki/Galois/Counter_Mode
	//https://golang.org/pkg/crypto/cipher/#NewGCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	//Create a nonce. Nonce should be from GCM
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}

	//Encrypt the data using aesGCM.Seal
	//Since we don't want to save the nonce somewhere else in this case, we add it as a prefix to the encrypted data. The first nonce argument in Seal is the prefix.
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return fmt.Sprintf("%x", ciphertext)
}

// Decrypt string
func Decrypt(encryptedString string, keyString string) (decryptedString string) {
	key, _ := hex.DecodeString(keyString)
	enc, _ := hex.DecodeString(encryptedString)

	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}

	//Create a new GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	//Get the nonce size
	nonceSize := aesGCM.NonceSize()

	//Extract the nonce from the encrypted data
	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]

	//Decrypt the data
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		panic(err.Error())
	}

	return fmt.Sprintf("%s", plaintext)
}

// GenerateApiKey creates an opaque, unique api key identifier for a credential. It is a
// random value encrypted under secretKey; the resulting string is both stored and handed
// to the client, and is only ever compared verbatim as a lookup key (never decrypted), so
// it stays stable across SecretKey rotations.
func GenerateApiKey(secretKey string) string {
	return Encrypt(GetRandomSha256(), secretKey)
}

// GenerateApiSecret creates a new API secret. It returns the plaintext secret to hand to
// the client exactly once, and the ciphertext to persist. Authentication decrypts the
// stored ciphertext and compares it to the plaintext the client presents — so the stored
// value can be re-encrypted under a new SecretKey (rotation) without changing the secret
// the client holds.
func GenerateApiSecret(secretKey string) (plaintext string, ciphertext string) {
	plaintext = GetRandomSha256()
	return plaintext, Encrypt(plaintext, secretKey)
}

// DecryptSafe is a panic-safe wrapper around Decrypt. Decrypt panics on a malformed key
// or ciphertext, or when the GCM auth tag fails (e.g. the value was encrypted under a
// different key). DecryptSafe recovers from that and reports ok=false instead, so callers
// on the authentication path can treat an undecryptable value as an auth failure rather
// than crashing.
func DecryptSafe(encryptedString string, keyString string) (plaintext string, ok bool) {
	if encryptedString == "" {
		return "", false
	}
	defer func() {
		if r := recover(); r != nil {
			plaintext = ""
			ok = false
		}
	}()
	return Decrypt(encryptedString, keyString), true
}

// IsValidAESHexKey reports whether keyString is a hex-encoded AES key of a valid
// length (16, 24 or 32 bytes → AES-128/192/256). Encrypt/Decrypt panic on an
// invalid key, so callers doing key rotation should validate up front.
func IsValidAESHexKey(keyString string) bool {
	key, err := hex.DecodeString(keyString)
	if err != nil {
		return false
	}
	switch len(key) {
	case 16, 24, 32:
		return true
	default:
		return false
	}
}

// ReEncrypt decrypts ciphertext with oldKey and re-encrypts the resulting
// plaintext with newKey. It is the primitive used by SecretKey rotation to move
// stored secrets from one AES key to another without exposing the plaintext.
//
// It returns (newCiphertext, true) only when the value was successfully
// decrypted with oldKey. Empty input, or a value that cannot be decrypted with
// oldKey (e.g. legacy plaintext, or a row already encrypted with newKey), yields
// ("", false) so the caller can leave the stored value untouched rather than
// corrupting it. Decrypt panics on malformed input; that panic is recovered here
// and reported as a failure.
func ReEncrypt(ciphertext, oldKey, newKey string) (result string, ok bool) {
	if ciphertext == "" {
		return "", false
	}
	defer func() {
		if r := recover(); r != nil {
			result = ""
			ok = false
		}
	}()
	plaintext := Decrypt(ciphertext, oldKey)
	return Encrypt(plaintext, newKey), true
}
