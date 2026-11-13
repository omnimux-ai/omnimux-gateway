package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercise real initiation rather than constructing a trusted digest in tests.
func startOAuthLoginTestFlow(t *testing.T, provider string) (string, *http.Cookie) {
	t.Helper()
	response := securityEnrollmentRequest(http.MethodPost, "/api/oauth/state", `{"provider":"`+provider+`","intent":"login"}`, "", service.AuthIdentity{}, GenerateOAuthCode)
	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Token string `json:"flow_token"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	require.True(t, result.Success, response.Body.String())
	require.NotEmpty(t, result.Data.Token)
	cookies := response.Result().Cookies()
	require.Len(t, cookies, 1)
	require.NotEmpty(t, cookies[0].Value)
	assert.NotContains(t, response.Body.String(), cookies[0].Value)
	flow, err := model.GetAuthFlow(result.Data.Token, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
	require.NoError(t, err)
	assert.NotEmpty(t, flow.SessionId)
	assert.NotEqual(t, cookies[0].Value, flow.SessionId)
	assert.NotContains(t, flow.Payload, cookies[0].Value)
	return result.Data.Token, cookies[0]
}

func oauthBrowserTestCallback(state, suffix string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	router := gin.New()
	router.GET("/api/oauth/:provider", HandleOAuth)
	request := httptest.NewRequest(http.MethodGet, "/api/oauth/auth-flow-test?state="+state+suffix, nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestOAuthLoginBrowserRejectsUncorrelatedCallbacks(t *testing.T) {
	for _, test := range []string{"missing", "other browser", "tampered", "empty", "duplicate", "legacy unbound", "expired", "wrong provider"} {
		t.Run(test, func(t *testing.T) {
			provider := setupAuthFlowControllerTest(t)
			state, cookie := startOAuthLoginTestFlow(t, "auth-flow-test")
			flow, err := model.GetAuthFlow(state, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
			require.NoError(t, err)
			cookies := []*http.Cookie{cookie}
			switch test {
			case "missing":
				cookies = nil
			case "other browser":
				_, otherCookie := startOAuthLoginTestFlow(t, "auth-flow-test")
				otherCookie.Name = cookie.Name
				cookies = []*http.Cookie{otherCookie}
			case "tampered":
				cookie.Value += "x"
			case "empty":
				cookie.Value = ""
			case "duplicate":
				cookies = append(cookies, cookie)
			case "legacy unbound":
				require.NoError(t, model.UpdateAuthFlowFields(flow.Id, map[string]interface{}{"session_id": ""}))
			case "expired":
				require.NoError(t, model.UpdateAuthFlowFields(flow.Id, map[string]interface{}{"expires_at": time.Now().Add(-time.Minute)}))
			case "wrong provider":
				require.NoError(t, model.UpdateAuthFlowFields(flow.Id, map[string]interface{}{"provider": "another-provider"}))
			}
			for _, suffix := range []string{"&code=local-code", "&error=access_denied"} {
				response := oauthBrowserTestCallback(state, suffix, cookies...)
				assert.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
				assert.Empty(t, response.Result().Cookies())
			}
			assert.Zero(t, provider.exchangeCalls)
			assert.Zero(t, provider.userInfoCalls)
			var stored model.AuthFlow
			require.NoError(t, model.DB.First(&stored, flow.Id).Error)
			assert.Nil(t, stored.ConsumedAt)
			for _, table := range []interface{}{&model.User{}, &model.UserSession{}} {
				var count int64
				require.NoError(t, model.DB.Model(table).Count(&count).Error)
				assert.Zero(t, count)
			}
			var factors int64
			require.NoError(t, model.DB.Model(&model.AuthFlow{}).Where("purpose <> ?", model.AuthFlowPurposeOAuth).Count(&factors).Error)
			assert.Zero(t, factors)
		})
	}
}

func TestOAuthLoginBrowserCookiePolicyAndConcurrentAttempts(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "secure"}[secure], func(t *testing.T) {
			provider := setupAuthFlowControllerTest(t)
			previous := common.SessionCookieSecure
			common.SessionCookieSecure = secure
			t.Cleanup(func() { common.SessionCookieSecure = previous })
			stateA, cookieA := startOAuthLoginTestFlow(t, "auth-flow-test")
			stateB, cookieB := startOAuthLoginTestFlow(t, "auth-flow-test")
			require.NotEqual(t, cookieA.Name, cookieB.Name)
			assert.NotEqual(t, cookieA.Value, cookieB.Value)
			for _, cookie := range []*http.Cookie{cookieA, cookieB} {
				assert.Equal(t, secure, strings.HasPrefix(cookie.Name, "__Host-"))
				assert.Equal(t, secure, cookie.Secure)
				assert.True(t, cookie.HttpOnly)
				assert.Equal(t, "/", cookie.Path)
				assert.Empty(t, cookie.Domain)
				assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
				assert.Positive(t, cookie.MaxAge)
				assert.LessOrEqual(t, cookie.MaxAge, 600)
				assert.WithinDuration(t, time.Now().Add(10*time.Minute), cookie.Expires, 2*time.Second)
			}
			if secure {
				fallback := *cookieA
				fallback.Name = strings.TrimPrefix(fallback.Name, "__Host-")
				response := oauthBrowserTestCallback(stateA, "&error=access_denied", &fallback)
				assert.Equal(t, http.StatusForbidden, response.Code)
			}
			// Both initial requests had no cookie: neither outstanding flow is overwritten.
			for _, pair := range []struct {
				state  string
				cookie *http.Cookie
			}{{stateA, cookieA}, {stateB, cookieB}} {
				response := oauthBrowserTestCallback(pair.state, "&error=access_denied", cookieA, cookieB)
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				cleared := response.Result().Cookies()
				require.Len(t, cleared, 1)
				assert.Equal(t, pair.cookie.Name, cleared[0].Name)
				assert.Negative(t, cleared[0].MaxAge)
				assert.Empty(t, cleared[0].Value)
				assert.Equal(t, secure, cleared[0].Secure)
				assert.Equal(t, "/", cleared[0].Path)
				_, err := model.GetAuthFlow(pair.state, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
				assert.ErrorIs(t, err, model.ErrAuthFlowConsumed)
				assert.Equal(t, http.StatusForbidden, oauthBrowserTestCallback(pair.state, "&code=local-code", pair.cookie).Code)
			}
			assert.Zero(t, provider.exchangeCalls)
		})
	}
}

func TestOAuthLoginBrowserRetryKeepsProof(t *testing.T) {
	provider := setupAuthFlowControllerTest(t)
	state, cookie := startOAuthLoginTestFlow(t, "auth-flow-test")
	provider.exchangeErr = errors.New("temporary local provider failure")
	response := oauthBrowserTestCallback(state, "&code=local-code", cookie)
	assert.Empty(t, response.Result().Cookies())
	_, err := model.GetAuthFlow(state, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
	require.NoError(t, err)
	require.Equal(t, 1, provider.exchangeCalls)
	provider.exchangeErr = nil
	previous := common.RegisterEnabled
	common.RegisterEnabled = false
	t.Cleanup(func() { common.RegisterEnabled = previous })
	response = oauthBrowserTestCallback(state, "&code=local-code", cookie)
	assert.Equal(t, 2, provider.exchangeCalls)
	assert.Equal(t, 1, provider.userInfoCalls)
	cleared := response.Result().Cookies()
	require.Len(t, cleared, 1)
	assert.Equal(t, cookie.Name, cleared[0].Name)
	assert.Negative(t, cleared[0].MaxAge)
	_, err = model.GetAuthFlow(state, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
	assert.ErrorIs(t, err, model.ErrAuthFlowConsumed)
}

func TestOAuthLoginBrowserProofPrecedesSessionAndSecondFactor(t *testing.T) {
	for _, factor := range []bool{false, true} {
		t.Run(map[bool]string{false: "session", true: "second factor"}[factor], func(t *testing.T) {
			user, _ := setupSecurityEnrollmentTest(t)
			provider := &boundLoginOAuthProvider{userID: user.Id}
			oauth.Register("auth-flow-test", provider)
			t.Cleanup(func() { oauth.Unregister("auth-flow-test") })
			// A GitHub-column binding authenticates only through its proven
			// subject claim, so the fixture carries the binding and the claim.
			linkProvenGitHubBinding(t, user, "auth-flow-test", "external-user")
			if factor {
				newSecurityLoginPasskey(t, user.Id)
			}
			state, cookie := startOAuthLoginTestFlow(t, "auth-flow-test")
			var sessionsBefore int64
			require.NoError(t, model.DB.Model(&model.UserSession{}).Count(&sessionsBefore).Error)
			wrong := oauthBrowserTestCallback(state, "&code=local-code")
			assert.Equal(t, http.StatusForbidden, wrong.Code)
			assert.Empty(t, wrong.Result().Cookies())
			var sessionsAfter int64
			require.NoError(t, model.DB.Model(&model.UserSession{}).Count(&sessionsAfter).Error)
			assert.Equal(t, sessionsBefore, sessionsAfter)
			var challenges int64
			require.NoError(t, model.DB.Model(&model.AuthFlow{}).Where("purpose = ?", model.AuthFlowPurposeLoginVerification).Count(&challenges).Error)
			assert.Zero(t, challenges)
			assert.Zero(t, provider.exchangeCalls)
			success := oauthBrowserTestCallback(state, "&code=local-code", cookie)
			var body securityEnrollmentResponse
			require.NoError(t, common.Unmarshal(success.Body.Bytes(), &body))
			require.True(t, body.Success, success.Body.String())
			refresh := false
			cleared := false
			for _, returned := range success.Result().Cookies() {
				if returned.Name == service.RefreshCookieName {
					refresh = true
				}
				if returned.Name == cookie.Name {
					cleared = returned.MaxAge < 0
				}
			}
			assert.Equal(t, !factor, refresh)
			assert.True(t, cleared)
			require.NoError(t, model.DB.Model(&model.AuthFlow{}).Where("purpose = ?", model.AuthFlowPurposeLoginVerification).Count(&challenges).Error)
			if factor {
				assert.EqualValues(t, 1, challenges)
			} else {
				assert.Zero(t, challenges)
			}
			assert.Equal(t, http.StatusForbidden, oauthBrowserTestCallback(state, "&code=local-code", cookie).Code)
			assert.Equal(t, 1, provider.exchangeCalls)
		})
	}
}

func TestOAuthLoginBrowserProofPrecedesSignup(t *testing.T) {
	_, _ = setupSecurityEnrollmentTest(t)
	provider := &authFlowTestOAuthProvider{}
	oauth.Register("auth-flow-test", provider)
	t.Cleanup(func() { oauth.Unregister("auth-flow-test") })
	previous := common.RegisterEnabled
	common.RegisterEnabled = true
	t.Cleanup(func() { common.RegisterEnabled = previous })
	state, cookie := startOAuthLoginTestFlow(t, "auth-flow-test")
	var before int64
	require.NoError(t, model.DB.Model(&model.User{}).Count(&before).Error)
	assert.Equal(t, http.StatusForbidden, oauthBrowserTestCallback(state, "&code=local-code").Code)
	var after int64
	require.NoError(t, model.DB.Model(&model.User{}).Count(&after).Error)
	assert.Equal(t, before, after)
	response := oauthBrowserTestCallback(state, "&code=local-code", cookie)
	var body securityEnrollmentResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	require.True(t, body.Success, response.Body.String())
	require.NoError(t, model.DB.Model(&model.User{}).Count(&after).Error)
	assert.Equal(t, before+1, after)
}
