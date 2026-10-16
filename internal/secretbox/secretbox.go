// Package secretbox encrypts small secrets for storage, such as Gmail refresh tokens (docs/adr/0006):
// AES-256-GCM, with a key from config and a random nonce per secret.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
)

type Box struct{ aead cipher.AEAD }

// New reads a key as config holds it: 32 random bytes in base64 (`openssl rand -base64 32`).
func New(key string) (*Box, error) {
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(raw) != 32 {
		return nil, errors.New("the key must be 32 bytes in base64: openssl rand -base64 32")
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Seal encrypts plaintext; the nonce goes in front of the result.
func (b *Box) Seal(plaintext []byte) []byte {
	nonce := make([]byte, b.aead.NonceSize())
	_, _ = rand.Read(nonce) // crypto/rand.Read never fails
	return b.aead.Seal(nonce, nonce, plaintext, nil)
}

// Open decrypts what Seal made with the same key, and fails on anything else.
func (b *Box) Open(sealed []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n {
		return nil, errors.New("too short to be sealed")
	}
	return b.aead.Open(nil, sealed[:n], sealed[n:], nil)
}
