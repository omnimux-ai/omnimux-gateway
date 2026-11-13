package advancedcustom

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountMonitorTikTokObservedProfileContract(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		valid      bool
	}{
		{"five leaves", `{"code":200,"data":{"userInfo":{"user":{"nickname":"Fixture Creator","signature":"Fixture biography","avatarLarger":"https://example.com/avatar.jpg"},"stats":{"followerCount":0,"videoCount":2}}}}`, true},
		{"nickname only", `{"data":{"userInfo":{"user":{"nickname":"Synthetic"},"stats":{}}}}`, true},
		{"signature only", `{"data":{"userInfo":{"user":{"signature":"Synthetic bio"},"stats":{}}}}`, true},
		{"avatar only", `{"data":{"userInfo":{"user":{"avatarLarger":"HTTPS://example.invalid/avatar.png"},"stats":{}}}}`, true},
		{"zero followers", `{"data":{"userInfo":{"user":{},"stats":{"followerCount":0}}}}`, true},
		{"zero videos", `{"data":{"userInfo":{"user":{},"stats":{"videoCount":0}}}}`, true},
		{"negative followers valid videos", `{"data":{"userInfo":{"user":{},"stats":{"followerCount":-1,"videoCount":2}}}}`, true},
		{"all blank", `{"data":{"userInfo":{"user":{"nickname":"  ","signature":"\t","avatarLarger":"  "},"stats":{}}}}`, false},
		{"negative counters", `{"data":{"userInfo":{"user":{},"stats":{"followerCount":-1,"videoCount":-2}}}}`, false},
		{"nonfinite string counters", `{"data":{"userInfo":{"user":{},"stats":{"followerCount":"Infinity","videoCount":"NaN"}}}}`, false},
		{"overflowing numeric counter", `{"data":{"userInfo":{"user":{},"stats":{"followerCount":1e999}}}}`, false},
		{"numeric string counters are unobserved", `{"data":{"userInfo":{"user":{},"stats":{"followerCount":"0","videoCount":"2"}}}}`, false},
		{"bad avatar scheme", `{"data":{"userInfo":{"user":{"avatarLarger":"file:///synthetic/avatar.png"},"stats":{}}}}`, false},
		{"avatar without host", `{"data":{"userInfo":{"user":{"avatarLarger":"https://"},"stats":{}}}}`, false},
		{"avatar without hostname", `{"data":{"userInfo":{"user":{"avatarLarger":"https://:443/avatar.png"},"stats":{}}}}`, false},
		{"missing user", `{"data":{"userInfo":{"stats":{"followerCount":0}}}}`, false},
		{"missing stats", `{"data":{"userInfo":{"user":{"nickname":"Synthetic"}}}}`, false},
		{"wrong user", `{"data":{"userInfo":{"user":[],"stats":{"followerCount":0}}}}`, false},
		{"wrong stats", `{"data":{"userInfo":{"user":{"nickname":"Synthetic"},"stats":null}}}`, false},
		{"unknown leaf", `{"data":{"userInfo":{"user":{"name":"Synthetic"},"stats":{"video_count":0}}}}`, false},
		{"unknown wrapper", `{"data":{"unknown":{"userInfo":{"user":{"nickname":"Synthetic"},"stats":{}}}}}`, false},
		{"outer failure", `{"code":500,"data":{"userInfo":{"user":{"nickname":"Synthetic"},"stats":{}}}}`, false},
		{"wrapper failure blocks fallback", `{"data":{"nickname":"Old synthetic","userInfo":{"success":false,"user":{"nickname":"Synthetic"},"stats":{}}}}`, false},
		{"user failure blocks fallback", `{"data":{"nickname":"Old synthetic","userInfo":{"user":{"ok":false,"nickname":"Synthetic"},"stats":{}}}}`, false},
		{"stats failure blocks fallback", `{"data":{"nickname":"Old synthetic","userInfo":{"user":{"nickname":"Synthetic"},"stats":{"code":500,"videoCount":0}}}}`, false},
		{"incomplete failure blocks fallback", `{"data":{"nickname":"Old synthetic","userInfo":{"user":{"success":false,"nickname":"Synthetic"}}}}`, false},
		{"old selected failure blocks new wrapper", `{"data":{"user":{"ok":false,"nickname":"Old synthetic"},"userInfo":{"user":{"nickname":"Synthetic"},"stats":{}}}}`, false},
		{"malformed wrapper preserves old legal shape", `{"data":{"nickname":"Old synthetic","userInfo":{"user":[],"stats":null}}}`, true},
		{"old profile unchanged", `{"data":{"user":{"nickname":"Old synthetic","follower_count":0}}}`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder := shapeContext()
			require.True(t, EnableAccountMonitorShapeDiagnostic(c, "tiktok-user"))
			adaptor, info := monitorResponseAdaptor(t, "tiktok-user", "tikhub-user")
			usage, apiErr := adaptor.DoResponse(c, shapeResponse(tt.body), info)
			if tt.valid {
				require.Nil(t, apiErr)
				actual, ok := usage.(*dto.Usage)
				require.True(t, ok)
				assert.Equal(t, 1, actual.TotalTokens)
				assert.Equal(t, tt.body, recorder.Body.String())
				assert.Nil(t, GetAccountMonitorShapeEvidence(c))
			} else {
				require.NotNil(t, apiErr)
				assert.Nil(t, usage)
				assert.Empty(t, recorder.Body.String())
			}
		})
	}
}

func TestAccountMonitorTikTokObservedProfileDoesNotExpandOtherModels(t *testing.T) {
	const body = `{"data":{"userInfo":{"user":{"nickname":"Synthetic","signature":"Synthetic bio","avatarLarger":"https://example.invalid/avatar.png"},"stats":{"followerCount":0,"videoCount":0}}}}`
	_, err := ValidateAccountMonitorResponse("tikhub-user", []byte(body))
	require.Error(t, err, "the legacy model-less contract must remain unchanged")
	for _, model := range []string{"x-user", "tiktok-posts", "x-posts"} {
		policy := "tikhub-user"
		if model == "tiktok-posts" || model == "x-posts" {
			policy = "tikhub-posts"
		}
		c, recorder := shapeContext()
		adaptor, info := monitorResponseAdaptor(t, model, policy)
		usage, apiErr := adaptor.DoResponse(c, shapeResponse(body), info)
		require.NotNil(t, apiErr)
		assert.Nil(t, usage)
		assert.Empty(t, recorder.Body.String())
	}
}
