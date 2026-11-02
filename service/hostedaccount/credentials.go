package hostedaccount

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"io"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// CredentialPlain is the in-memory vault payload. Never serialize to API DTOs.
type CredentialPlain struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	TOTPSecret    string `json:"totp_secret"`
	RecoveryEmail string `json:"recovery_email"`
}

func credKeyBytes() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("HOSTED_CRED_KEY"))
	if raw == "" {
		return nil, ErrCredKeyMissing
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, ErrCredKeyMissing
	}
	return key, nil
}

// Seal encrypts plain credentials with AES-256-GCM.
// Stored form is hex(nonce || ciphertext||tag).
func Seal(plain CredentialPlain) (string, error) {
	key, err := credKeyBytes()
	if err != nil {
		return "", err
	}
	raw, err := common.Marshal(plain)
	if err != nil {
		return "", err
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
	sealed := aead.Seal(nil, nonce, raw, nil)
	out := make([]byte, 0, len(nonce)+len(sealed))
	out = append(out, nonce...)
	out = append(out, sealed...)
	return hex.EncodeToString(out), nil
}

// Open decrypts a CredentialsEnc hex envelope. Wrong key / truncated blob → ErrCredInvalid.
func Open(enc string) (CredentialPlain, error) {
	var empty CredentialPlain
	key, err := credKeyBytes()
	if err != nil {
		return empty, err
	}
	blob, err := hex.DecodeString(strings.TrimSpace(enc))
	if err != nil || len(blob) == 0 {
		return empty, ErrCredInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return empty, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return empty, err
	}
	ns := aead.NonceSize()
	if len(blob) < ns+aead.Overhead() {
		return empty, ErrCredInvalid
	}
	plainBytes, err := aead.Open(nil, blob[:ns], blob[ns:], nil)
	if err != nil {
		return empty, ErrCredInvalid
	}
	var out CredentialPlain
	if err := common.Unmarshal(plainBytes, &out); err != nil {
		return empty, ErrCredInvalid
	}
	return out, nil
}
