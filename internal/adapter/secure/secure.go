// Package secure provides password hashing, secret encryption and random tokens.
package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Passwords hashes with bcrypt.
type Passwords struct{}

// Hash returns a bcrypt hash of the password.
func (Passwords) Hash(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("bcrypt: %w", err)
	}
	return string(h), nil
}

// Matches reports whether password matches hash.
func (Passwords) Matches(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// Tokens makes 256-bit URL-safe random tokens.
type Tokens struct{}

// New returns a fresh random token.
func (Tokens) New() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// ErrBadMasterKey means TABLETOP_SECRET_KEY is missing or the wrong size.
var ErrBadMasterKey = errors.New("master key must be 32 bytes, base64 encoded (generate with: openssl rand -base64 32)")

// Vault encrypts secrets with AES-256-GCM.
type Vault struct {
	aead cipher.AEAD
}

// NewVault builds a vault from a base64-encoded 32-byte master key.
func NewVault(masterKeyB64 string) (*Vault, error) {
	key, err := base64.StdEncoding.DecodeString(masterKeyB64)
	if err != nil || len(key) != 32 {
		return nil, ErrBadMasterKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &Vault{aead: aead}, nil
}

// Seal encrypts plaintext; the random nonce is prepended to the ciphertext.
func (v *Vault) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	return v.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// ErrTampered means a sealed value could not be authenticated.
var ErrTampered = errors.New("sealed value is corrupt or was encrypted with a different master key")

// Open decrypts a value produced by Seal.
func (v *Vault) Open(sealed []byte) ([]byte, error) {
	n := v.aead.NonceSize()
	if len(sealed) < n {
		return nil, ErrTampered
	}
	plain, err := v.aead.Open(nil, sealed[:n], sealed[n:], nil)
	if err != nil {
		return nil, ErrTampered
	}
	return plain, nil
}
