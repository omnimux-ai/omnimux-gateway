package advancedcustom

import (
	"errors"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountMonitorProbePrivateModelAndPolicyBoundary(t *testing.T) {
	assert.False(t, EnableAccountMonitorProbe(nil, "x-user", "tikhub-user"))
	for _, capability := range []struct{ model, policy string }{
		{"x-user", "tikhub-user"}, {"tiktok-user", "tikhub-user"},
		{"x-posts", "tikhub-posts"}, {"tiktok-posts", "tikhub-posts"},
	} {
		c, _ := shapeContext()
		require.True(t, EnableAccountMonitorProbe(c, capability.model, capability.policy))
		assert.True(t, accountMonitorProbeEnabled(c, capability.model, capability.policy))
		assert.Nil(t, GetAccountMonitorShapeEvidence(c))
		assert.Nil(t, accountMonitorShapeCollectorFromContext(c))
		assert.False(t, accountMonitorProbeEnabled(c, capability.model, ""))
		assert.False(t, accountMonitorProbeEnabled(c, "other", capability.policy))
	}
	for _, capability := range []struct{ model, policy string }{
		{"other", "tikhub-user"}, {"x-user", "tikhub-posts"},
		{"tiktok-posts", "tikhub-user"}, {"x-user", " tikhub-user"},
		{"x-user ", "tikhub-user"}, {"x-posts", ""},
	} {
		c, _ := shapeContext()
		assert.False(t, EnableAccountMonitorProbe(c, capability.model, capability.policy))
		assert.False(t, accountMonitorProbeEnabled(c, capability.model, capability.policy))
	}
}

func TestAccountMonitorOrdinaryProbeBodyBoundaryAndPrivateFailures(t *testing.T) {
	base := `{"data":{"items":[]}}`
	for _, extra := range []int{0, 1} {
		c, recorder := shapeContext()
		require.True(t, EnableAccountMonitorProbe(c, "tiktok-posts", "tikhub-posts"))
		adaptor, info := monitorResponseAdaptor(t, "tiktok-posts", "tikhub-posts")
		body := base + strings.Repeat(" ", accountMonitorShapeMaxBody+extra-len(base))
		reader := &shapeReadCloser{reader: strings.NewReader(body)}
		response := shapeResponse("")
		response.Body = reader
		usage, apiErr := adaptor.DoResponse(c, response, info)
		assert.Equal(t, 1, reader.closes)
		assert.Equal(t, len(body), reader.read)
		assert.Nil(t, GetAccountMonitorShapeEvidence(c))
		if extra == 0 {
			assert.Nil(t, apiErr)
			assert.NotNil(t, usage)
			assert.Equal(t, body, recorder.Body.String())
		} else {
			require.NotNil(t, apiErr)
			assert.Equal(t, "account monitoring diagnostic response exceeded limit", apiErr.Error())
			assert.True(t, types.IsSkipRetryError(apiErr))
			assert.Nil(t, usage)
			assert.Empty(t, recorder.Body.String())
		}
	}
	c, recorder := shapeContext()
	require.True(t, EnableAccountMonitorProbe(c, "tiktok-posts", "tikhub-posts"))
	adaptor, info := monitorResponseAdaptor(t, "tiktok-posts", "tikhub-posts")
	reader := &shapeReadCloser{readErr: errors.New("private-reader-sentinel"), closeErr: errors.New("private-close-sentinel")}
	response := shapeResponse("")
	response.Body = reader
	usage, apiErr := adaptor.DoResponse(c, response, info)
	require.NotNil(t, apiErr)
	assert.Equal(t, "account monitoring diagnostic response read failed", apiErr.Error())
	assert.NotContains(t, apiErr.Error(), "sentinel")
	assert.Equal(t, 1, reader.closes)
	assert.Nil(t, usage)
	assert.Nil(t, GetAccountMonitorShapeEvidence(c))
	assert.Empty(t, recorder.Body.String())
}

func TestAccountMonitorProbeCannotBeEnabledByRequestOrMismatchedBinding(t *testing.T) {
	for _, scenario := range []string{"request headers and body", "forged context value", "nil context value", "different actual model", "different actual policy"} {
		t.Run(scenario, func(t *testing.T) {
			c, recorder := shapeContext()
			c.Request.Header.Set(accountMonitorProbeContextKey, "true")
			c.Request.Header.Set("account_monitor_probe", "true")
			c.Request.Body = shapeResponse(`{"account_monitor_probe":true}`).Body
			switch scenario {
			case "forged context value":
				c.Set(accountMonitorProbeContextKey, map[string]string{"model": "x-user", "policy": "tikhub-user"})
			case "nil context value":
				c.Set(accountMonitorProbeContextKey, (*accountMonitorProbeContext)(nil))
			case "different actual model":
				require.True(t, EnableAccountMonitorProbe(c, "tiktok-user", "tikhub-user"))
			case "different actual policy":
				require.True(t, EnableAccountMonitorProbe(c, "x-user", "tikhub-user"))
			}
			policy := "tikhub-user"
			if scenario == "different actual policy" {
				policy = ""
			}
			adaptor, info := monitorResponseAdaptor(t, "x-user", policy)
			body := `{"data":{"nickname":"Fixture","items":[]}}` + strings.Repeat(" ", accountMonitorShapeMaxBody+1)
			reader := &shapeReadCloser{reader: strings.NewReader(body)}
			response := shapeResponse("")
			response.Body = reader
			usage, apiErr := adaptor.DoResponse(c, response, info)
			assert.Nil(t, apiErr)
			assert.NotNil(t, usage)
			assert.Equal(t, len(body), reader.read)
			assert.Equal(t, body, recorder.Body.String())
			assert.Nil(t, GetAccountMonitorShapeEvidence(c))
		})
	}
}
