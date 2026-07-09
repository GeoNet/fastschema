package authservice

import "strings"

// IsLocalLoginDisabled reports whether password and OTP login are turned off,
// leaving only federated providers. The local provider object is still
// registered; this flag only gates the login/OTP and self-service surfaces.
func (as *AuthService) IsLocalLoginDisabled() bool {
	cfg := as.AppConfig()
	return cfg != nil && cfg.AuthConfig != nil && cfg.AuthConfig.DisableLocalLogin
}

// isAdminEmail reports whether email is on the configured admin allowlist. The
// comparison is case-insensitive and trims surrounding whitespace. Used to grant
// the admin role on first federated login.
func (as *AuthService) isAdminEmail(email string) bool {
	cfg := as.AppConfig()
	if cfg == nil || cfg.AuthConfig == nil {
		return false
	}

	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return false
	}

	for _, e := range cfg.AuthConfig.AdminEmails {
		if strings.ToLower(strings.TrimSpace(e)) == email {
			return true
		}
	}

	return false
}
