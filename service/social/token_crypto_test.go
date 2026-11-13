package social

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testEncKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestSealOpenTokenRoundtrip(t *testing.T) {
	k := testEncKey(t)
	enc, err := SealToken("access-token-abc", k)
	require.NoError(t, err)
	assert.NotContains(t, enc, "access-token-abc")
	plain, err := OpenToken(enc, k)
	require.NoError(t, err)
	assert.Equal(t, "access-token-abc", plain)
}

func TestSealTokenEmptyPlain(t *testing.T) {
	k := testEncKey(t)
	enc, err := SealToken("", k)
	require.NoError(t, err)
	assert.Equal(t, "", enc)
}

func TestSealTokenKeyMissing(t *testing.T) {
	_, err := SealToken("x", "")
	require.ErrorIs(t, err, ErrTokenEncKeyMissing)
}

func TestSealTokenKeyBadBase64(t *testing.T) {
	_, err := SealToken("x", "not-base64!!!")
	require.ErrorIs(t, err, ErrTokenEncKeyMissing)
}

func TestSealTokenKeyWrongLen(t *testing.T) {
	_, err := SealToken("x", base64.StdEncoding.EncodeToString([]byte("short")))
	require.ErrorIs(t, err, ErrTokenEncKeyMissing)
}

func TestOpenTokenTampered(t *testing.T) {
	k := testEncKey(t)
	enc, err := SealToken("secret", k)
	require.NoError(t, err)
	b := []byte(enc)
	b[len(b)-1] ^= 0x01
	_, err = OpenToken(string(b), k)
	require.Error(t, err)
}

func TestOpenTokenWrongKey(t *testing.T) {
	k1 := testEncKey(t)
	other := make([]byte, 32)
	for i := range other {
		other[i] = byte(255 - i)
	}
	k2 := base64.StdEncoding.EncodeToString(other)
	enc, err := SealToken("secret", k1)
	require.NoError(t, err)
	_, err = OpenToken(enc, k2)
	require.ErrorIs(t, err, ErrTokenEncInvalid)
}
