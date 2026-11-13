package oauth

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

func redirectURIForHost(t *testing.T, host, slug string) string {
	t.Helper()
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "https://"+host+"/api/oauth/"+slug, nil)
	return OAuthRedirectURI(context, slug)
}

func TestOAuthRedirectURIUsesTrustedRequestHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := system_setting.ServerAddress
	t.Cleanup(func() { system_setting.ServerAddress = previous })
	system_setting.ServerAddress = "https://omnimux.ai"

	cases := []struct {
		host string
		want string
	}{
		{"omnimux.ai", "https://omnimux.ai/oauth/oidc"},
		{"d.omnimux.ai", "https://d.omnimux.ai/oauth/oidc"},
		{"www.omnimux.ai", "https://www.omnimux.ai/oauth/oidc"},
		{"evil.example.com", "https://omnimux.ai/oauth/oidc"},
		{"notomnimux.ai", "https://omnimux.ai/oauth/oidc"},
	}
	for _, testCase := range cases {
		if got := redirectURIForHost(t, testCase.host, "oidc"); got != testCase.want {
			t.Fatalf("host %s: got %s want %s", testCase.host, got, testCase.want)
		}
	}
}

func TestOAuthRedirectURIKeepsLocalPort(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := system_setting.ServerAddress
	t.Cleanup(func() { system_setting.ServerAddress = previous })
	system_setting.ServerAddress = "http://127.0.0.1:3000"

	if got := redirectURIForHost(t, "127.0.0.1:3000", "oidc"); got != "http://127.0.0.1:3000/oauth/oidc" {
		t.Fatalf("got %s", got)
	}
}
