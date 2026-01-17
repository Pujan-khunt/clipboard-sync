// Package crypto implements the security layer for the clipboard synchronization usecase.
// It uses AES-256-GCM for authenticated encryption and SHA-256 for key derivation
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

// DeriveKey turns a string password into a 32-byte key using SHA-256
func DeriveKey(password string) []byte {
	hash := sha256.Sum256([]byte(password))
	return hash[:]
}

// Encrypt encrypts data using AES-GCM
// It returns a byte slice containing [Nonce (12b)] + [Ciphertext]
func Encrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Create a random nonce value using the rand package.
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts the data using AES-GCM
// It expects input in the format [Nonce (12b)] + [Ciphertext]
func Decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Split nonce and actual ciphertext
	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	return gcm.Open(nil, nonce, actualCiphertext, nil)
}
