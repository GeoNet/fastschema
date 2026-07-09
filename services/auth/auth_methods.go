package authservice

import (
	"github.com/fastschema/fastschema/fs"
	"github.com/fastschema/fastschema/pkg/auth"
)

// methodsDTO is the public, whitelisted shape of the enabled login methods. It
// is an explicit allowlist: ONLY provider names and the local/otp flags are
// exposed. The raw AuthConfig.Providers map holds client_id/client_secret, so
// the config object must NEVER be serialized here. The method list is public
// knowledge (it is rendered on the login screen); the secrets are not.
type methodsDTO struct {
	Providers []string `json:"providers"`
	// Local reports whether the username/password form should be shown. When
	// false the dash renders social buttons only.
	Local bool `json:"local"`
	OTP   bool `json:"otp"`
}

// AuthMethods returns the enabled login methods so the dash can render the
// correct buttons. Public and always available (not gated by the cli flag).
func (as *AuthService) AuthMethods(c fs.Context, _ any) (*methodsDTO, error) {
	dto := &methodsDTO{Providers: []string{}, Local: true}

	cfg := as.AppConfig()
	if cfg == nil || cfg.AuthConfig == nil {
		return dto, nil
	}

	// "local" is an internal provider entry, not a social button; never expose it
	// in the provider list (the dash decides the local form via the Local flag).
	for _, p := range cfg.AuthConfig.EnabledProviders {
		if p == auth.ProviderLocal {
			continue
		}
		dto.Providers = append(dto.Providers, p)
	}

	localDisabled := cfg.AuthConfig.DisableLocalLogin
	dto.Local = !localDisabled
	// OTP is an email-only single factor, so it is disabled together with local
	// login when the deployment requires federated (MFA-at-provider) sign-in.
	dto.OTP = !localDisabled && cfg.AuthConfig.OTP != nil && cfg.AuthConfig.OTP.Enabled

	return dto, nil
}
