package social

// fork: tiktok-direct — AES-256-GCM at-rest encryption for TikTok OAuth tokens.
// Mirrors hostedaccount/credentials.go pattern; key from env SOCIAL_TOKEN_ENC_KEY (base64 32B).

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

var (
	ErrTokenEncKeyMissing = errors.New("social token encryption key is not configured")
	ErrTokenEncInvalid    = errors.New("social token encryption payload is invalid")
)

// tokenEncKeyBytes parses cfg.SocialTokenEncKey.
func tokenEncKeyBytes(encKeyB64 string) ([]byte, error) {
	raw := strings.TrimSpace(encKeyB64)
	if raw == "" {
		return nil, ErrTokenEncKeyMissing
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, ErrTokenEncKeyMissing
	}
	return key, nil
}

// SealToken encrypts a plaintext token. Stored form is hex(nonce || ciphertext||tag).
func SealToken(plain, encKeyB64 string) (string, error) {
	key, err := tokenEncKeyBytes(encKeyB64)
	if err != nil {
		return "", err
	}
	if plain == "" {
		return "", nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nil, nonce, []byte(plain), nil)
	out := make([]byte, 0, len(nonce)+len(sealed))
	out = append(out, nonce...)
	out = append(out, sealed...)
	return hex.EncodeToString(out), nil
}

// OpenToken decrypts a token produced by SealToken.
func OpenToken(enc, encKeyB64 string) (string, error) {
	key, err := tokenEncKeyBytes(encKeyB64)
	if err != nil {
		return "", err
	}
	blob, err := hex.DecodeString(strings.TrimSpace(enc))
	if err != nil || len(blob) == 0 {
		return "", ErrTokenEncInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := aead.NonceSize()
	if len(blob) < ns+aead.Overhead() {
		return "", ErrTokenEncInvalid
	}
	plain, err := aead.Open(nil, blob[:ns], blob[ns:], nil)
	if err != nil {
		return "", ErrTokenEncInvalid
	}
	return string(plain), nil
}
