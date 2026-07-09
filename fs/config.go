package fs

import (
	"github.com/fastschema/fastschema/db"
	"github.com/fastschema/fastschema/logger"
)

type Config struct {
	Dir                    string                        `json:"dir"`
	AppName                string                        `json:"app_name"`
	AppKey                 string                        `json:"app_key"`
	BrandName              string                        `json:"brand_name"`        // White-label brand name shown in the dash; empty falls back to AppName
	BrandDescription       string                        `json:"brand_description"` // Tagline shown under the brand name
	BrandLogo              string                        `json:"brand_logo"`        // Logo as URL, data-URI, or inline SVG (sanitized client-side)
	BrandFavicon           string                        `json:"brand_favicon"`     // Favicon as URL, data-URI, or inline SVG (sanitized client-side)
	Port                   string                        `json:"port"`
	BaseURL                string                        `json:"base_url"`
	DashURL                string                        `json:"dash_url"`
	APIBaseName            string                        `json:"api_base_name"`
	DashBaseName           string                        `json:"dash_base_name"`
	Logger                 logger.Logger                 `json:"-"`
	LoggerConfig           *logger.Config                `json:"logger_config"` // If Logger is set, LoggerConfig will be ignored
	DB                     db.Client                     `json:"-"`
	DBConfig               *db.Config                    `json:"db_config"` // If DB is set, DBConfig will be ignored
	StorageConfig          *StorageConfig                `json:"storage_config"`
	AuthConfig             *AuthConfig                   `json:"auth_config"`
	MailConfig             *MailConfig                   `json:"mail_config"`
	RolePermissionSettings *RolePermissionSettingsConfig `json:"role_permission_settings"`
	AuditConfig            *AuditConfig                  `json:"audit_config"`
	SystemSchemas          []any                         `json:"-"` // types to build the system schemas
	Hooks                  *Hooks                        `json:"-"`
	HideResourcesInfo      bool                          `json:"hide_resources_info"`
	MaxRequestBodySize     int                           `json:"max_request_body_size"` // in bytes, default is 4MB
}

func (ac *Config) Clone() *Config {
	c := &Config{
		Dir:                ac.Dir,
		AppName:            ac.AppName,
		AppKey:             ac.AppKey,
		BrandName:          ac.BrandName,
		BrandDescription:   ac.BrandDescription,
		BrandLogo:          ac.BrandLogo,
		BrandFavicon:       ac.BrandFavicon,
		Port:               ac.Port,
		BaseURL:            ac.BaseURL,
		DashURL:            ac.DashURL,
		APIBaseName:        ac.APIBaseName,
		DashBaseName:       ac.DashBaseName,
		Logger:             ac.Logger,
		DB:                 ac.DB,
		HideResourcesInfo:  ac.HideResourcesInfo,
		SystemSchemas:      append([]any{}, ac.SystemSchemas...),
		MaxRequestBodySize: ac.MaxRequestBodySize,
	}

	if ac.DBConfig != nil {
		c.DBConfig = ac.DBConfig.Clone()
	}

	if ac.StorageConfig != nil {
		c.StorageConfig = ac.StorageConfig.Clone()
	}

	if ac.AuthConfig != nil {
		c.AuthConfig = ac.AuthConfig.Clone()
	}

	if ac.MailConfig != nil {
		c.MailConfig = ac.MailConfig.Clone()
	}

	if ac.LoggerConfig != nil {
		c.LoggerConfig = ac.LoggerConfig.Clone()
	}

	if ac.Hooks != nil {
		c.Hooks = ac.Hooks.Clone()
	}

	if ac.RolePermissionSettings != nil {
		c.RolePermissionSettings = ac.RolePermissionSettings.Clone()
	}

	if ac.AuditConfig != nil {
		c.AuditConfig = ac.AuditConfig.Clone()
	}

	return c
}
