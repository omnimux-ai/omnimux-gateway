package controller

import (
	"context"
	"net/http/httptest"
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

// The fixtures impersonate the built-in providers so the identity column, the
// registry slug and the release path stay the ones production uses.
const (
	legacyBindingSlug = "github"
	nonHazardSlug     = "discord"
)

type legacyBindingProvider struct {
	subject  string
	username string
	legacyID string
}

func (*legacyBindingProvider) GetName() string { return "Legacy Binding" }
func (*legacyBindingProvider) IsEnabled() bool { return true }
func (*legacyBindingProvider) ExchangeToken(context.Context, string, *gin.Context) (*oauth.OAuthToken, error) {
	return &oauth.OAuthToken{}, nil
}

// GetUserInfo reports the immutable subject and the mutable login together,
// exactly like the GitHub provider does.
func (p *legacyBindingProvider) GetUserInfo(context.Context, *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	extra := map[string]any{}
	if p.legacyID != "" {
		extra["legacy_id"] = p.legacyID
	}
	return &oauth.OAuthUser{ProviderUserID: p.subject, Username: p.username, DisplayName: p.username, Extra: extra}, nil
}
func (*legacyBindingProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsGitHubIdAlreadyTaken(providerUserID)
}
func (*legacyBindingProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.GitHubId = providerUserID
	return user.FillUserByGitHubId()
}
func (*legacyBindingProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.GitHubId = providerUserID
}
func (*legacyBindingProvider) GetProviderPrefix() string    { return "legacy_" }
func (*legacyBindingProvider) ProviderUserIDColumn() string { return oauth.GitHubIdentityColumn }

// discordLikeProvider stands for a provider whose identity column never held a
// mutable name; its lookup semantics must stay exactly as they were.
type discordLikeProvider struct{ legacyBindingProvider }

func (*discordLikeProvider) ProviderUserIDColumn() string { return "discord_id" }
func (*discordLikeProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsDiscordIdAlreadyTaken(providerUserID)
}
func (*discordLikeProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.DiscordId = providerUserID
	return user.FillUserByDiscordId()
}
func (*discordLikeProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.DiscordId = providerUserID
}

func setupLegacyBindingTest(t *testing.T) (*model.User, service.AuthIdentity) {
	t.Helper()
	return setupSecurityEnrollmentTest(t)
}

// registerProvider swaps a registry entry for the duration of one test and puts
// the original back, so a fixture can carry a real provider slug.
func registerProvider(t *testing.T, slug string, provider oauth.Provider) {
	t.Helper()
	previous := oauth.GetProvider(slug)
	oauth.Register(slug, provider)
	t.Cleanup(func() {
		if previous != nil {
			oauth.Register(slug, previous)
			return
		}
		oauth.Unregister(slug)
	})
}

type legacyBindingLoginResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		User struct {
			Id       int    `json:"id"`
			GitHubId string `json:"github_id"`
		} `json:"user"`
	} `json:"data"`
}

// oauthLogin starts a real login flow so the callback carries the browser
// binding the flow issues, then returns the callback response.
func oauthLogin(t *testing.T, slug string) *httptest.ResponseRecorder {
	t.Helper()
	state, cookie := startOAuthLoginTestFlow(t, slug)
	router := gin.New()
	router.GET("/api/oauth/:provider", HandleOAuth)
	request := httptest.NewRequest("GET", "/api/oauth/"+slug+"?state="+state+"&code=provider-code", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func legacyBindingClaims(t *testing.T) []model.ExternalIdentityClaim {
	t.Helper()
	var claims []model.ExternalIdentityClaim
	require.NoError(t, model.DB.Order("id").Find(&claims).Error)
	return claims
}

func storedGitHubId(t *testing.T, userId int) string {
	t.Helper()
	user, err := model.GetUserById(userId, false)
	require.NoError(t, err)
	return user.GitHubId
}

func newLegacyBindingUser(t *testing.T, username string) *model.User {
	t.Helper()
	user := &model.User{Username: username, Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: username, AuthVersion: 1}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

// A stored value in the legacy-hazard column is a recovery record, not an
// ownership proof: neither the mutable login nor a numeric login that happens to
// equal the incoming subject may resolve a login.
func TestLegacyOAuthLoginNeverSubstitutesAStoredValueForSubjectOwnership(t *testing.T) {
	for _, scenario := range []struct {
		name         string
		stored       string
		subject      string
		legacyID     string
		registration bool
	}{
		{"mutable username matches the stored value", "octocat", "900001", "octocat", true},
		{"mutable username with registration disabled", "octocat", "900002", "octocat", false},
		{"numeric username equals the incoming subject", "42", "42", "", true},
		{"numeric username equals the incoming subject with registration disabled", "42", "42", "", false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			victim, _ := setupLegacyBindingTest(t)
			require.NoError(t, model.DB.Model(victim).Update("github_id", scenario.stored).Error)
			previousRegister := common.RegisterEnabled
			common.RegisterEnabled = scenario.registration
			t.Cleanup(func() { common.RegisterEnabled = previousRegister })
			sessionsBefore, err := model.CountActiveUserSessions(victim.Id, time.Now().Unix())
			require.NoError(t, err)
			registerProvider(t, legacyBindingSlug, &legacyBindingProvider{subject: scenario.subject, username: "attacker", legacyID: scenario.legacyID})

			response := oauthLogin(t, legacyBindingSlug)
			var result legacyBindingLoginResult
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))

			assert.Equal(t, scenario.stored, storedGitHubId(t, victim.Id), "the legacy value must survive as a recovery record")
			assert.NotEqual(t, victim.Id, result.Data.User.Id, "a stored value must never authenticate the callback")
			sessionsAfter, err := model.CountActiveUserSessions(victim.Id, time.Now().Unix())
			require.NoError(t, err)
			assert.Equal(t, sessionsBefore, sessionsAfter, "the victim must not receive a session")
			for _, claim := range legacyBindingClaims(t) {
				assert.NotEqual(t, victim.Id, claim.UserId, "no claim may be minted for the legacy owner")
			}
			if scenario.registration {
				require.True(t, result.Success, response.Body.String())
				assert.NotZero(t, result.Data.User.Id, "registration still creates the caller's own account")
			} else {
				assert.False(t, result.Success, response.Body.String())
			}
		})
	}
}

// A proven claim still logs in, and login must not rewrite the stored subject
// just because the provider's mutable login changed.
func TestLegacyOAuthLoginAcceptsAProvenSubjectClaim(t *testing.T) {
	user, _ := setupLegacyBindingTest(t)
	require.NoError(t, model.DB.Model(user).Update("github_id", "777").Error)
	require.NoError(t, model.ClaimExternalIdentityWithTx(model.DB, legacyBindingSlug, "777", user.Id))
	registerProvider(t, legacyBindingSlug, &legacyBindingProvider{subject: "777", username: "renamed-since-binding", legacyID: "renamed-since-binding"})

	response := oauthLogin(t, legacyBindingSlug)
	var result legacyBindingLoginResult
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	require.True(t, result.Success, response.Body.String())
	assert.Equal(t, user.Id, result.Data.User.Id)
	assert.Equal(t, "777", storedGitHubId(t, user.Id), "a proven login must not rewrite the binding")
}

// A claim whose stored binding no longer matches fails closed instead of
// authenticating stale ownership.
func TestLegacyOAuthLoginFailsClosedWhenAClaimAndItsBindingDisagree(t *testing.T) {
	user, _ := setupLegacyBindingTest(t)
	require.NoError(t, model.ClaimExternalIdentityWithTx(model.DB, legacyBindingSlug, "888", user.Id))
	registerProvider(t, legacyBindingSlug, &legacyBindingProvider{subject: "888"})

	response := oauthLogin(t, legacyBindingSlug)
	var result legacyBindingLoginResult
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	assert.False(t, result.Success, response.Body.String())
	assert.NotEqual(t, user.Id, result.Data.User.Id)
	assert.Empty(t, storedGitHubId(t, user.Id))
}

// The authenticated rebind is the recovery path: the rightful owner can prove a
// subject that an unproven legacy row already holds, and the recovery record is
// preserved rather than overwritten.
func TestLegacyOAuthRebindRecoversAnAccountHoldingAnUnprovenLegacyValue(t *testing.T) {
	user, identity := setupLegacyBindingTest(t)
	require.NoError(t, model.DB.Model(user).Update("github_id", "octocat").Error)
	holder := newLegacyBindingUser(t, "legacy-holder")
	require.NoError(t, model.DB.Model(holder).Update("github_id", "555").Error)
	registerProvider(t, legacyBindingSlug, &legacyBindingProvider{subject: "555", username: "owner"})

	started := legacyBindingStartFlow(t, identity)
	callback := legacyBindingCallback(t, started.FlowToken, identity)
	require.True(t, callback.Success, callback.Message)

	assert.Equal(t, "555", storedGitHubId(t, user.Id))
	assert.Equal(t, "555", storedGitHubId(t, holder.Id), "an unproven recovery record is never overwritten")
	claims := legacyBindingClaims(t)
	require.Len(t, claims, 1)
	assert.Equal(t, legacyBindingSlug, claims[0].Provider)
	assert.Equal(t, "555", claims[0].Subject)
	assert.Equal(t, user.Id, claims[0].UserId)

	// The recovered account now logs in through the proven subject.
	login := oauthLogin(t, legacyBindingSlug)
	var result legacyBindingLoginResult
	require.NoError(t, common.Unmarshal(login.Body.Bytes(), &result))
	require.True(t, result.Success, login.Body.String())
	assert.Equal(t, user.Id, result.Data.User.Id)
}

// Rebinding replaces the claim, so the previous subject is released and cannot
// authenticate later, and a subject another user already proved stays reserved.
func TestLegacyOAuthRebindReleasesThePreviousClaimAndRejectsAProvenCollision(t *testing.T) {
	user, identity := setupLegacyBindingTest(t)
	require.NoError(t, model.DB.Model(user).Update("github_id", "111").Error)
	require.NoError(t, model.ClaimExternalIdentityWithTx(model.DB, legacyBindingSlug, "111", user.Id))
	other := newLegacyBindingUser(t, "proven-owner")
	require.NoError(t, model.DB.Model(other).Update("github_id", "222").Error)
	require.NoError(t, model.ClaimExternalIdentityWithTx(model.DB, legacyBindingSlug, "222", other.Id))

	registerProvider(t, legacyBindingSlug, &legacyBindingProvider{subject: "222"})
	started := legacyBindingStartFlow(t, identity)
	callback := legacyBindingCallback(t, started.FlowToken, identity)
	assert.False(t, callback.Success, callback.Message)
	assert.Equal(t, "111", storedGitHubId(t, user.Id), "a refused bind must not mutate the caller")
	assert.Equal(t, "222", storedGitHubId(t, other.Id))
	assert.Len(t, legacyBindingClaims(t), 2, "the refused bind must leave both claims intact")

	// Rebinding to a free subject succeeds and releases the replaced claim.
	registerProvider(t, legacyBindingSlug, &legacyBindingProvider{subject: "333"})
	started = legacyBindingStartFlow(t, identity)
	callback = legacyBindingCallback(t, started.FlowToken, identity)
	require.True(t, callback.Success, callback.Message)
	assert.Equal(t, "333", storedGitHubId(t, user.Id))
	var released int64
	require.NoError(t, model.DB.Model(&model.ExternalIdentityClaim{}).Where("subject = ?", "111").Count(&released).Error)
	assert.Zero(t, released, "a replaced claim must not stay reserved")
	registerProvider(t, legacyBindingSlug, &legacyBindingProvider{subject: "111"})
	login := oauthLogin(t, legacyBindingSlug)
	var result legacyBindingLoginResult
	require.NoError(t, common.Unmarshal(login.Body.Bytes(), &result))
	assert.NotEqual(t, user.Id, result.Data.User.Id, "a released subject must not authenticate its former owner")
	assert.Equal(t, "333", storedGitHubId(t, user.Id))
}

// Unbind releases the claim, a soft delete makes it unusable, and an unproven
// stored value is never offered as a security-proof factor.
func TestLegacyOAuthUnbindDeleteAndProofFactorStayConsistent(t *testing.T) {
	t.Run("unbind releases the claim", func(t *testing.T) {
		user, _ := setupLegacyBindingTest(t)
		require.NoError(t, model.DB.Model(user).Update("github_id", "444").Error)
		require.NoError(t, model.ClaimExternalIdentityWithTx(model.DB, legacyBindingSlug, "444", user.Id))
		require.NoError(t, user.ClearBinding("github"))
		assert.Empty(t, storedGitHubId(t, user.Id))
		assert.Empty(t, legacyBindingClaims(t), "an unbound subject must not stay reserved")
	})

	t.Run("soft delete keeps the claim but refuses the login", func(t *testing.T) {
		user, identity := setupLegacyBindingTest(t)
		require.NoError(t, model.DB.Model(user).Update("github_id", "445").Error)
		require.NoError(t, model.ClaimExternalIdentityWithTx(model.DB, legacyBindingSlug, "445", user.Id))
		_, deleteErr := model.DeleteUserForSession(identity)
		require.NoError(t, deleteErr)
		assert.Len(t, legacyBindingClaims(t), 1)
		registerProvider(t, legacyBindingSlug, &legacyBindingProvider{subject: "445"})
		response := oauthLogin(t, legacyBindingSlug)
		var result legacyBindingLoginResult
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
		assert.False(t, result.Success, response.Body.String())
		assert.NotZero(t, result.Message)
	})

	t.Run("an unproven binding is not a proof factor", func(t *testing.T) {
		user, identity := setupLegacyBindingTest(t)
		require.NoError(t, model.DB.Model(user).Updates(map[string]any{"password": "", "github_id": "446"}).Error)
		registerProvider(t, legacyBindingSlug, &legacyBindingProvider{subject: "446"})
		requirements, err := service.GetVerificationRequirements(identity, service.VerificationScopeTwoFASetup)
		require.NoError(t, err)
		assert.Empty(t, requirements.OAuthProviders, "an unproven stored value cannot verify itself")

		require.NoError(t, model.ClaimExternalIdentityWithTx(model.DB, legacyBindingSlug, "446", user.Id))
		requirements, err = service.GetVerificationRequirements(identity, service.VerificationScopeTwoFASetup)
		require.NoError(t, err)
		require.Len(t, requirements.OAuthProviders, 1)
		assert.Equal(t, legacyBindingSlug, requirements.OAuthProviders[0].Slug)
		assert.Equal(t, "446", requirements.OAuthProviders[0].UserID)
	})
}

// A provider whose identity column never held a mutable name keeps its existing
// lookup: the stored value still resolves the login with no claim.
func TestLegacyOAuthNonHazardProviderLookupIsUnchanged(t *testing.T) {
	user, _ := setupLegacyBindingTest(t)
	require.NoError(t, model.DB.Model(user).Update("discord_id", "immutable-subject").Error)
	registerProvider(t, nonHazardSlug, &discordLikeProvider{legacyBindingProvider{subject: "immutable-subject"}})

	response := oauthLogin(t, nonHazardSlug)
	var result legacyBindingLoginResult
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	require.True(t, result.Success, response.Body.String())
	assert.Equal(t, user.Id, result.Data.User.Id)
	assert.Empty(t, legacyBindingClaims(t))
}

type legacyBindingFlowStart struct {
	FlowToken string `json:"flow_token"`
}

func legacyBindingStartFlow(t *testing.T, identity service.AuthIdentity) legacyBindingFlowStart {
	t.Helper()
	operation := service.VerificationOperation{Scope: service.VerificationScopeAccountBind, Context: []byte(`{"provider":"` + legacyBindingSlug + `"}`)}
	proof := issueSecurityEnrollmentProof(t, identity, operation, service.VerificationMethodPassword)
	response := securityEnrollmentRequest("POST", "/api/oauth/state", `{"provider":"`+legacyBindingSlug+`","intent":"bind"}`, proof, identity, GenerateOAuthCode)
	var result securityEnrollmentResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	require.True(t, result.Success, response.Body.String())
	var started legacyBindingFlowStart
	require.NoError(t, common.Unmarshal(result.Data, &started))
	return started
}

func legacyBindingCallback(t *testing.T, flowToken string, identity service.AuthIdentity) legacyBindingLoginResult {
	t.Helper()
	handler := func(c *gin.Context) {
		c.Params = gin.Params{{Key: "provider", Value: legacyBindingSlug}}
		HandleOAuth(c)
	}
	response := securityEnrollmentRequest("GET", "/api/oauth/"+legacyBindingSlug+"?state="+flowToken+"&code=provider-code", "", "", identity, handler)
	var result legacyBindingLoginResult
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	return result
}
