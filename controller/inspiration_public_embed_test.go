package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Only a real TikTok player address may be published: imported rows that never
// produced one have no playback surface at all, and anything else in that field
// would be injected straight into an iframe on a public page.
func TestInspirationPublicListPublishesOnlyValidEmbedUrls(t *testing.T) {
	fixture := `{"success":true,"data":{"total":4,"page":1,"size":30,"items":[
 {"id":1,"type":"video","analysis":{"embed_player_url":"https://www.tiktok.com/player/v1/7535235994035997966"}},
 {"id":2,"type":"video","analysis":{"embed_player_url":"javascript:alert(1)"}},
 {"id":3,"type":"video","analysis":{"embed_player_url":"https://evil.example.com/player/v1/7535235994035997966"}},
 {"id":4,"type":"video","analysis":{"embed_player_url":null}}
]}}`
	publicUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixture))
	})

	rec := callPublicList(t, "")
	require.Equal(t, http.StatusOK, rec.Code)

	var payload publicEnvelope
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 4)

	require.NotNil(t, payload.Data.Items[0].EmbedURL)
	assert.Equal(t, "https://www.tiktok.com/player/v1/7535235994035997966", *payload.Data.Items[0].EmbedURL)
	assert.Nil(t, payload.Data.Items[1].EmbedURL, "a javascript: payload is never published")
	assert.Nil(t, payload.Data.Items[2].EmbedURL, "a foreign host is never published")
	assert.Nil(t, payload.Data.Items[3].EmbedURL, "null upstream stays null")

	assert.NotContains(t, rec.Body.String(), "javascript:", "the raw analysis payload is never projected")
}

func TestInspirationPublicEmbedURLValidation(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"player v1", "https://www.tiktok.com/player/v1/7535235994035997966", true},
		{"bare host", "https://tiktok.com/player/v2/123", true},
		{"http scheme", "http://www.tiktok.com/player/v1/123", false},
		{"no digits", "https://www.tiktok.com/player/v1/abc", false},
		{"no version", "https://www.tiktok.com/player/1/123", false},
		{"extra segment", "https://www.tiktok.com/player/v1/123/embed", false},
		{"other path", "https://www.tiktok.com/@creator/video/123", false},
		{"subdomain spoof", "https://www.tiktok.com.evil.example/player/v1/123", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := inspirationPublicEmbedURL(map[string]any{"embed_player_url": tc.raw})
			if tc.want {
				require.NotNil(t, got)
				assert.Equal(t, tc.raw, *got)
				return
			}
			assert.Nil(t, got)
		})
	}
}
