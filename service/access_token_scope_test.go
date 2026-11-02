package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfluencerCreationGrantIsPersonalAndExplicit(t *testing.T) {
	requested := []string{"influencer_creation:read", "influencer_creation:write"}
	granted, err := NormalizeAccessTokenScopes(0, common.RoleCommonUser, requested)
	require.NoError(t, err)
	assert.Equal(t, requested, granted)
	assert.False(t, AccessTokenScopeGranted([]string{"influencer:read", "usage:read", "influencer_admin:write"}, requested[0]))
	for _, group := range AccessTokenCatalog(0, common.RoleCommonUser) {
		for _, resource := range group.Resources {
			if resource.Resource != "influencer_creation" {
				continue
			}
			assert.Equal(t, AccessTokenGroupPersonal, group.Group)
			require.Len(t, resource.Actions, 2)
			assert.Equal(t, "View your characters, reference-video creations, and their task history.", resource.Actions[0].DescriptionKey)
			assert.Equal(t, "Create characters and videos using your account credits or subscription allowance.", resource.Actions[1].DescriptionKey)
			return
		}
	}
	t.Fatal("creation grant is missing from the personal permissions catalog")
}
