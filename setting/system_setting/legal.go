package system_setting

import "github.com/QuantumNous/new-api/setting/config"

type LegalSettings struct {
	UserAgreement  string `json:"user_agreement"`
	PrivacyPolicy  string `json:"privacy_policy"`
	// RequireConsent: when true and at least one document is non-empty,
	// login/register UI shows a checkbox and blocks submit until checked.
	// Default false: keep document content without forcing consent.
	RequireConsent bool `json:"require_consent"`
}

var defaultLegalSettings = LegalSettings{
	UserAgreement:  "",
	PrivacyPolicy:  "",
	RequireConsent: false,
}

func init() {
	config.GlobalConfig.Register("legal", &defaultLegalSettings)
}

func GetLegalSettings() *LegalSettings {
	return &defaultLegalSettings
}
