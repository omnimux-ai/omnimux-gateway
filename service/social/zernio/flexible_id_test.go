package zernio

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlexibleID_UnmarshalStringAndObject(t *testing.T) {
	var a Account
	require.NoError(t, common.Unmarshal([]byte(`{
		"_id":"acc1",
		"profileId":"prof1",
		"platform":"tiktok",
		"username":"u1"
	}`), &a))
	assert.Equal(t, "acc1", a.ID)
	assert.Equal(t, "prof1", a.ProfileID.String())

	var b Account
	require.NoError(t, common.Unmarshal([]byte(`{
		"_id":"acc2",
		"profileId":{"_id":"prof2","name":"omnimux_u_1"},
		"platform":"tiktok",
		"username":"geminix56",
		"displayName":"GeminiX"
	}`), &b))
	assert.Equal(t, "acc2", b.ID)
	assert.Equal(t, "prof2", b.ProfileID.String())
	assert.Equal(t, "GeminiX", b.DisplayName)
}
