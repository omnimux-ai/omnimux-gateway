package common

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRedirectURL(t *testing.T) {
	// Save original trusted domains and restore after test
	originalDomains := constant.TrustedRedirectDomains
	defer func() {
		constant.TrustedRedirectDomains = originalDomains
	}()

	tests := []struct {
		name           string
		url            string
		trustedDomains []string
		wantErr        bool
		errContains    string
	}{
		// Valid cases
		{
			name:           "exact domain match with https",
			url:            "https://example.com/success",
			trustedDomains: []string{"example.com"},
			wantErr:        false,
		},
		{
			name:           "exact domain match with http",
			url:            "http://example.com/callback",
			trustedDomains: []string{"example.com"},
			wantErr:        false,
		},
		{
			name:           "subdomain match",
			url:            "https://sub.example.com/success",
			trustedDomains: []string{"example.com"},
			wantErr:        false,
		},
		{
			name:           "case insensitive domain",
			url:            "https://EXAMPLE.COM/success",
			trustedDomains: []string{"example.com"},
			wantErr:        false,
		},

		// Invalid cases - untrusted domain
		{
			name:           "untrusted domain",
			url:            "https://evil.com/phishing",
			trustedDomains: []string{"example.com"},
			wantErr:        true,
			errContains:    "not in the trusted domains list",
		},
		{
			name:           "suffix attack - fakeexample.com",
			url:            "https://fakeexample.com/success",
			trustedDomains: []string{"example.com"},
			wantErr:        true,
			errContains:    "not in the trusted domains list",
		},
		{
			name:           "empty trusted domains list",
			url:            "https://example.com/success",
			trustedDomains: []string{},
			wantErr:        true,
			errContains:    "not in the trusted domains list",
		},

		// Invalid cases - scheme
		{
			name:           "javascript scheme",
			url:            "javascript:alert('xss')",
			trustedDomains: []string{"example.com"},
			wantErr:        true,
			errContains:    "invalid URL scheme",
		},
		{
			name:           "data scheme",
			url:            "data:text/html,<script>alert('xss')</script>",
			trustedDomains: []string{"example.com"},
			wantErr:        true,
			errContains:    "invalid URL scheme",
		},

		// Edge cases
		{
			name:           "empty URL",
			url:            "",
			trustedDomains: []string{"example.com"},
			wantErr:        true,
			errContains:    "invalid URL scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up trusted domains for this test case
			constant.TrustedRedirectDomains = tt.trustedDomains

			err := ValidateRedirectURL(tt.url)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateRedirectURL(%q) expected error containing %q, got nil", tt.url, tt.errContains)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateRedirectURL(%q) error = %q, want error containing %q", tt.url, err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateRedirectURL(%q) unexpected error: %v", tt.url, err)
				}
			}
		})
	}
}

func resetSessionCookieSettingsAfterTest(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		SessionCookieSecure = false
		SessionCookieTrustedURLs = nil
	})
}

func TestInitSessionCookieSettingsDefaultsToInsecure(t *testing.T) {
	resetSessionCookieSettingsAfterTest(t)
	t.Setenv("SESSION_COOKIE_SECURE", "")
	t.Setenv("SESSION_COOKIE_TRUSTED_URL", "")

	require.NoError(t, InitSessionCookieSettings())
	assert.False(t, SessionCookieSecure)
	assert.Empty(t, SessionCookieTrustedURLs)
}

func TestInitSessionCookieSettingsRequiresBothEnvVars(t *testing.T) {
	t.Run("secure without trusted url", func(t *testing.T) {
		resetSessionCookieSettingsAfterTest(t)
		t.Setenv("SESSION_COOKIE_SECURE", "true")
		t.Setenv("SESSION_COOKIE_TRUSTED_URL", "")

		require.Error(t, InitSessionCookieSettings())
	})

	t.Run("trusted url without secure", func(t *testing.T) {
		resetSessionCookieSettingsAfterTest(t)
		t.Setenv("SESSION_COOKIE_SECURE", "")
		t.Setenv("SESSION_COOKIE_TRUSTED_URL", "https://example.com")

		require.Error(t, InitSessionCookieSettings())
	})
}

func TestInitSessionCookieSettingsRequiresHTTPSURL(t *testing.T) {
	resetSessionCookieSettingsAfterTest(t)
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("SESSION_COOKIE_TRUSTED_URL", "http://example.com")

	require.Error(t, InitSessionCookieSettings())
}

func TestInitSessionCookieSettingsEnablesSecureCookie(t *testing.T) {
	resetSessionCookieSettingsAfterTest(t)
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("SESSION_COOKIE_TRUSTED_URL", "https://example.com")

	require.NoError(t, InitSessionCookieSettings())
	assert.True(t, SessionCookieSecure)
	assert.Equal(t, []string{"https://example.com"}, SessionCookieTrustedURLs)
}

func TestInitSessionCookieSettingsAllowsMultipleTrustedURLs(t *testing.T) {
	resetSessionCookieSettingsAfterTest(t)
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("SESSION_COOKIE_TRUSTED_URL", "https://example.com, https://admin.example.com")

	require.NoError(t, InitSessionCookieSettings())
	assert.True(t, SessionCookieSecure)
	assert.Equal(t, []string{"https://example.com", "https://admin.example.com"}, SessionCookieTrustedURLs)
}

func TestInitSessionCookieSettingsRejectsEmptyTrustedURLInList(t *testing.T) {
	resetSessionCookieSettingsAfterTest(t)
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("SESSION_COOKIE_TRUSTED_URL", "https://example.com,")

	require.Error(t, InitSessionCookieSettings())
}

func TestInitSessionCookieSettingsNormalizesExactOrigins(t *testing.T) {
	resetSessionCookieSettingsAfterTest(t)
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("SESSION_COOKIE_TRUSTED_URL", "https://EXAMPLE.com:443,https://admin.example.com:8443/")

	require.NoError(t, InitSessionCookieSettings())
	assert.Equal(t, []string{"https://example.com", "https://admin.example.com:8443"}, SessionCookieTrustedURLs)
}

func TestInitSessionCookieSettingsRejectsNonOriginURLs(t *testing.T) {
	for _, trustedURL := range []string{
		"https://*.example.com",
		"https://user@example.com",
		"https://example.com/admin",
		"https://example.com?next=admin",
		"https://example.com#admin",
	} {
		t.Run(trustedURL, func(t *testing.T) {
			resetSessionCookieSettingsAfterTest(t)
			t.Setenv("SESSION_COOKIE_SECURE", "true")
			t.Setenv("SESSION_COOKIE_TRUSTED_URL", trustedURL)
			require.Error(t, InitSessionCookieSettings())
		})
	}
}

func TestInitSessionCookieSettingsExplicitFalseAndCaseInsensitive(t *testing.T) {
	t.Run("explicit false without trusted url succeeds", func(t *testing.T) {
		resetSessionCookieSettingsAfterTest(t)
		t.Setenv("SESSION_COOKIE_SECURE", "false")
		t.Setenv("SESSION_COOKIE_TRUSTED_URL", "")

		require.NoError(t, InitSessionCookieSettings())
		assert.False(t, SessionCookieSecure)
		assert.Empty(t, SessionCookieTrustedURLs)
	})

	t.Run("explicit uppercase FALSE without trusted url succeeds", func(t *testing.T) {
		resetSessionCookieSettingsAfterTest(t)
		t.Setenv("SESSION_COOKIE_SECURE", "FALSE")
		t.Setenv("SESSION_COOKIE_TRUSTED_URL", "")

		require.NoError(t, InitSessionCookieSettings())
		assert.False(t, SessionCookieSecure)
		assert.Empty(t, SessionCookieTrustedURLs)
	})

	t.Run("explicit false with trusted url fails closed", func(t *testing.T) {
		resetSessionCookieSettingsAfterTest(t)
		t.Setenv("SESSION_COOKIE_SECURE", "false")
		t.Setenv("SESSION_COOKIE_TRUSTED_URL", "https://example.com")

		err := InitSessionCookieSettings()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SESSION_COOKIE_TRUSTED_URL requires SESSION_COOKIE_SECURE=true")
		assert.False(t, SessionCookieSecure)
		assert.Empty(t, SessionCookieTrustedURLs)
	})

	t.Run("uppercase TRUE with trusted url enables secure mode", func(t *testing.T) {
		resetSessionCookieSettingsAfterTest(t)
		t.Setenv("SESSION_COOKIE_SECURE", "TRUE")
		t.Setenv("SESSION_COOKIE_TRUSTED_URL", "https://example.com")

		require.NoError(t, InitSessionCookieSettings())
		assert.True(t, SessionCookieSecure)
		assert.Equal(t, []string{"https://example.com"}, SessionCookieTrustedURLs)
	})
}

func TestInitSessionCookieSettingsRejectsInvalidBooleanValues(t *testing.T) {
	for _, raw := range []string{"1", "0", "yes", "no", "on", "invalid"} {
		t.Run(raw, func(t *testing.T) {
			resetSessionCookieSettingsAfterTest(t)
			t.Setenv("SESSION_COOKIE_SECURE", raw)
			t.Setenv("SESSION_COOKIE_TRUSTED_URL", "https://example.com")

			err := InitSessionCookieSettings()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "SESSION_COOKIE_SECURE must be true or false")
			assert.False(t, SessionCookieSecure)
			assert.Empty(t, SessionCookieTrustedURLs)
		})
	}
}

func TestLogStartupSuccessSessionCookieWarning(t *testing.T) {
	previousSecure := SessionCookieSecure
	previousLanguage := DefaultLanguage
	LogWriterMu.Lock()
	previousWriter := gin.DefaultWriter
	LogWriterMu.Unlock()
	t.Cleanup(func() {
		SessionCookieSecure = previousSecure
		SetLogLanguage(previousLanguage)
		LogWriterMu.Lock()
		gin.DefaultWriter = previousWriter
		LogWriterMu.Unlock()
	})

	captureStartup := func(secure bool, lang string) string {
		SessionCookieSecure = secure
		SetLogLanguage(lang)
		var buf bytes.Buffer
		LogWriterMu.Lock()
		gin.DefaultWriter = &buf
		LogWriterMu.Unlock()
		LogStartupSuccess(time.Now(), "3000")
		return buf.String()
	}

	t.Run("insecure mode emits english warning with trusted url hint", func(t *testing.T) {
		out := captureStartup(false, "")
		assert.Contains(t, out, "Warning: Refresh cookie is not secure and refresh/logout Origin validation is disabled. Please set SESSION_COOKIE_SECURE=true and SESSION_COOKIE_TRUSTED_URL in production.")
	})

	t.Run("insecure mode emits zh-CN translated warning with trusted url hint", func(t *testing.T) {
		out := captureStartup(false, "zh-CN")
		assert.Contains(t, out, "警告：SESSION_COOKIE_SECURE 未开启")
		assert.Contains(t, out, "SESSION_COOKIE_SECURE=true")
		assert.Contains(t, out, "SESSION_COOKIE_TRUSTED_URL")
	})

	t.Run("secure mode suppresses insecure cookie warning", func(t *testing.T) {
		out := captureStartup(true, "")
		assert.NotContains(t, out, "SESSION_COOKIE_SECURE")
		assert.NotContains(t, out, "SESSION_COOKIE_TRUSTED_URL")
		assert.NotContains(t, out, "Refresh cookie is not secure")
	})
}
