package hostedaccount

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCredKey() string {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 3)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func TestSealOpenRoundTrip(t *testing.T) {
	t.Setenv("HOSTED_CRED_KEY", testCredKey())
	plain := CredentialPlain{
		Username:      "tiktok.user",
		Password:      "s3cret",
		TOTPSecret:    "JBSWY3DPEHPK3PXP",
		RecoveryEmail: "ops@example.com",
	}
	enc, err := Seal(plain)
	require.NoError(t, err)
	assert.NotEmpty(t, enc)
	assert.NotContains(t, enc, "s3cret")
	assert.NotContains(t, enc, "tiktok.user")

	got, err := Open(enc)
	require.NoError(t, err)
	assert.Equal(t, plain, got)
}

func TestOpenWrongKey(t *testing.T) {
	t.Setenv("HOSTED_CRED_KEY", testCredKey())
	enc, err := Seal(CredentialPlain{Username: "u", Password: "p"})
	require.NoError(t, err)

	other := make([]byte, 32)
	for i := range other {
		other[i] = 0xff
	}
	t.Setenv("HOSTED_CRED_KEY", base64.StdEncoding.EncodeToString(other))
	_, err = Open(enc)
	assert.ErrorIs(t, err, ErrCredInvalid)
}

func TestSealMissingKey(t *testing.T) {
	t.Setenv("HOSTED_CRED_KEY", "")
	_, err := Seal(CredentialPlain{Username: "u"})
	assert.ErrorIs(t, err, ErrCredKeyMissing)
}

func TestSealShortKey(t *testing.T) {
	t.Setenv("HOSTED_CRED_KEY", base64.StdEncoding.EncodeToString([]byte("tooshort")))
	_, err := Seal(CredentialPlain{Username: "u"})
	assert.ErrorIs(t, err, ErrCredKeyMissing)
}
